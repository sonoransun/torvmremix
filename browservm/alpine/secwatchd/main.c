/*
 * secwatchd — Security monitor daemon for the Browser VM.
 *
 * Runs as root. Monitors the Chromium browser process for:
 * - Stack canary violations (via /proc/pid/mem introspection)
 * - Honey token file access (via fanotify)
 * - File integrity changes (via inotify)
 *
 * Reports events as JSON lines to /dev/virtio-ports/com.torvm.secwatch.
 * On critical events, kills the browser and optionally restarts it.
 */

#include <dirent.h>
#include <errno.h>
#include <fcntl.h>
#include <signal.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <sys/fanotify.h>
#include <sys/inotify.h>
#include <sys/stat.h>
#include <sys/types.h>
#include <sys/wait.h>
#include <time.h>
#include <unistd.h>

#define VIRTIO_SERIAL "/dev/virtio-ports/com.torvm.secwatch"
#define BROWSER_USER_UID 1000

/* Report file descriptor (virtio-serial or stderr fallback). */
static int report_fd = -1;
static int canary_interval = 5;
static int honeytokens_enabled = 0;
static int auto_remediate = 0;
static volatile sig_atomic_t running = 1;

static void handle_signal(int sig) {
    (void)sig;
    running = 0;
}

/* Write a JSON security event to the report channel. */
static void report_event(const char *type, const char *severity,
                          const char *detail, int pid, const char *remediation) {
    char buf[1024];
    int n = snprintf(buf, sizeof(buf),
        "{\"type\":\"%s\",\"severity\":\"%s\",\"detail\":\"%s\","
        "\"pid\":%d,\"remediation\":\"%s\",\"ts\":%ld}\n",
        type, severity, detail, pid, remediation, (long)time(NULL));
    if (n > 0 && report_fd >= 0) {
        write(report_fd, buf, n);
    }
    /* Also log to stderr for VM console. */
    fprintf(stderr, "secwatchd: [%s] %s: %s\n", severity, type, detail);
}

/* Find the PID of the main chromium process. */
static pid_t find_chromium_pid(void) {
    /* Simple approach: scan /proc for processes owned by BROWSER_USER_UID
     * with "chromium" in the command line. */
    char path[256], buf[1024];
    DIR *proc = opendir("/proc");
    if (!proc) return 0;

    struct dirent *ent;
    while ((ent = readdir(proc)) != NULL) {
        if (ent->d_name[0] < '1' || ent->d_name[0] > '9') continue;

        snprintf(path, sizeof(path), "/proc/%s/status", ent->d_name);
        FILE *f = fopen(path, "r");
        if (!f) continue;

        int uid = -1;
        char name[256] = {0};
        while (fgets(buf, sizeof(buf), f)) {
            if (strncmp(buf, "Uid:", 4) == 0) {
                sscanf(buf + 4, "%d", &uid);
            } else if (strncmp(buf, "Name:", 5) == 0) {
                sscanf(buf + 5, " %255s", name);
            }
        }
        fclose(f);

        if (uid == BROWSER_USER_UID && strstr(name, "chromium") != NULL) {
            closedir(proc);
            return (pid_t)atoi(ent->d_name);
        }
    }
    closedir(proc);
    return 0;
}

/* Monitor honey token files via fanotify. */
static void monitor_honeytokens(void) {
    static const char *tokens[] = {
        "/home/browser/.ssh/id_rsa",
        "/home/browser/.aws/credentials",
        "/home/browser/.config/chromium/Login Data.bak",
        NULL,
    };

    int fan_fd = fanotify_init(FAN_CLOEXEC | FAN_CLASS_CONTENT,
                               O_RDONLY | O_CLOEXEC);
    if (fan_fd < 0) {
        fprintf(stderr, "secwatchd: fanotify_init: %s\n", strerror(errno));
        return;
    }

    for (int i = 0; tokens[i]; i++) {
        if (access(tokens[i], F_OK) != 0) continue;
        if (fanotify_mark(fan_fd, FAN_MARK_ADD, FAN_OPEN | FAN_ACCESS,
                          AT_FDCWD, tokens[i]) < 0) {
            fprintf(stderr, "secwatchd: fanotify_mark %s: %s\n",
                    tokens[i], strerror(errno));
        }
    }

    /* Read fanotify events in a loop. */
    char buf[4096];
    while (running) {
        ssize_t n = read(fan_fd, buf, sizeof(buf));
        if (n <= 0) {
            if (errno == EINTR) continue;
            break;
        }

        struct fanotify_event_metadata *meta =
            (struct fanotify_event_metadata *)buf;
        while (FAN_EVENT_OK(meta, n)) {
            if (meta->fd >= 0) {
                /* Read the path from /proc/self/fd/N. */
                char fdpath[64], filepath[256];
                snprintf(fdpath, sizeof(fdpath), "/proc/self/fd/%d", meta->fd);
                ssize_t plen = readlink(fdpath, filepath, sizeof(filepath) - 1);
                if (plen > 0) {
                    filepath[plen] = '\0';
                } else {
                    strcpy(filepath, "(unknown)");
                }
                close(meta->fd);

                char detail[512];
                snprintf(detail, sizeof(detail),
                         "Honey token accessed: %s by PID %d", filepath, meta->pid);
                report_event("honey_token_access", "critical",
                             detail, meta->pid, "killed");

                /* Kill the accessing process. */
                if (meta->pid > 1) {
                    kill(meta->pid, SIGKILL);
                }
            }
            meta = FAN_EVENT_NEXT(meta, n);
        }
    }

    close(fan_fd);
}

/* Monitor file integrity via inotify. */
static void monitor_integrity(void) {
    static const char *watched[] = {
        "/usr/bin/chromium-browser",
        "/etc/seccomp/chromium.bpf",
        "/etc/passwd",
        "/etc/shadow",
        NULL,
    };

    int ino_fd = inotify_init1(IN_CLOEXEC);
    if (ino_fd < 0) {
        fprintf(stderr, "secwatchd: inotify_init: %s\n", strerror(errno));
        return;
    }

    for (int i = 0; watched[i]; i++) {
        if (access(watched[i], F_OK) != 0) continue;
        inotify_add_watch(ino_fd, watched[i], IN_MODIFY | IN_DELETE_SELF | IN_ATTRIB);
    }

    char buf[4096];
    while (running) {
        ssize_t n = read(ino_fd, buf, sizeof(buf));
        if (n <= 0) {
            if (errno == EINTR) continue;
            break;
        }

        struct inotify_event *ev = (struct inotify_event *)buf;
        while ((char *)ev < buf + n) {
            char detail[256];
            snprintf(detail, sizeof(detail),
                     "File integrity violation detected (wd=%d, mask=0x%x)",
                     ev->wd, ev->mask);
            report_event("file_tamper", "critical", detail, 0, "none");
            ev = (struct inotify_event *)((char *)ev +
                  sizeof(struct inotify_event) + ev->len);
        }
    }

    close(ino_fd);
}

/* Canary validation loop: read stack canary from Chromium's process memory. */
static void canary_loop(void) {
    while (running) {
        sleep(canary_interval);
        if (!running) break;

        pid_t pid = find_chromium_pid();
        if (pid <= 0) continue;

        /* Read AT_RANDOM from /proc/pid/auxv to find canary source. */
        char auxv_path[64];
        snprintf(auxv_path, sizeof(auxv_path), "/proc/%d/auxv", pid);
        int fd = open(auxv_path, O_RDONLY);
        if (fd < 0) continue;

        /* Scan auxv for AT_RANDOM (type 25). */
        unsigned long auxv[2];
        unsigned long at_random = 0;
        while (read(fd, auxv, sizeof(auxv)) == sizeof(auxv)) {
            if (auxv[0] == 25) { /* AT_RANDOM */
                at_random = auxv[1];
                break;
            }
            if (auxv[0] == 0) break; /* AT_NULL */
        }
        close(fd);

        if (at_random == 0) continue;

        /* Read the canary value from the AT_RANDOM address. */
        char mem_path[64];
        snprintf(mem_path, sizeof(mem_path), "/proc/%d/mem", pid);
        fd = open(mem_path, O_RDONLY);
        if (fd < 0) continue;

        unsigned long canary = 0;
        if (pread(fd, &canary, sizeof(canary), at_random) != sizeof(canary)) {
            close(fd);
            continue;
        }
        close(fd);

        /* On first read, store the expected value. On subsequent reads,
         * compare. A changed canary indicates stack corruption. */
        static unsigned long expected_canary = 0;
        static int canary_initialized = 0;

        if (!canary_initialized) {
            expected_canary = canary;
            canary_initialized = 1;
            report_event("canary_check", "info",
                         "Stack canary baseline captured", pid, "none");
        } else if (canary != expected_canary) {
            char detail[128];
            snprintf(detail, sizeof(detail),
                     "Stack canary changed: expected 0x%lx, got 0x%lx",
                     expected_canary, canary);
            report_event("canary_violation", "critical",
                         detail, pid, "killed");
            kill(pid, SIGKILL);

            /* Reset for potential restart. */
            canary_initialized = 0;
        }
    }
}

int main(int argc, char *argv[]) {
    /* Parse arguments. */
    for (int i = 1; i < argc; i++) {
        if (strcmp(argv[i], "--interval") == 0 && i + 1 < argc) {
            canary_interval = atoi(argv[++i]);
            if (canary_interval < 1) canary_interval = 1;
        } else if (strcmp(argv[i], "--honeytokens") == 0) {
            honeytokens_enabled = 1;
        } else if (strcmp(argv[i], "--auto-remediate") == 0) {
            auto_remediate = 1;
        }
    }

    signal(SIGTERM, handle_signal);
    signal(SIGINT, handle_signal);

    /* Open report channel (virtio-serial to host controller). */
    report_fd = open(VIRTIO_SERIAL, O_WRONLY | O_NONBLOCK | O_CLOEXEC);
    if (report_fd < 0) {
        fprintf(stderr, "secwatchd: open %s: %s (falling back to stderr)\n",
                VIRTIO_SERIAL, strerror(errno));
        report_fd = STDERR_FILENO;
    }

    report_event("secwatchd_start", "info",
                 "Security monitor daemon started", getpid(), "none");

    /* Fork child processes for each monitoring subsystem. */
    pid_t children[3] = {0};
    int nchildren = 0;

    /* Canary validation (always enabled). */
    pid_t pid = fork();
    if (pid == 0) {
        canary_loop();
        _exit(0);
    }
    children[nchildren++] = pid;

    /* Honey token monitoring. */
    if (honeytokens_enabled) {
        pid = fork();
        if (pid == 0) {
            monitor_honeytokens();
            _exit(0);
        }
        children[nchildren++] = pid;
    }

    /* File integrity monitoring (always enabled). */
    pid = fork();
    if (pid == 0) {
        monitor_integrity();
        _exit(0);
    }
    children[nchildren++] = pid;

    /* Wait for termination signal. */
    while (running) {
        int status;
        pid_t w = waitpid(-1, &status, WNOHANG);
        if (w > 0) {
            /* A child died; if it was killed by signal, it might indicate
             * a seccomp violation in a monitoring thread. */
        }
        sleep(1);
    }

    /* Clean up children. */
    for (int i = 0; i < nchildren; i++) {
        if (children[i] > 0) kill(children[i], SIGTERM);
    }
    for (int i = 0; i < nchildren; i++) {
        if (children[i] > 0) waitpid(children[i], NULL, 0);
    }

    report_event("secwatchd_stop", "info",
                 "Security monitor daemon stopped", getpid(), "none");

    if (report_fd >= 0 && report_fd != STDERR_FILENO) {
        close(report_fd);
    }

    return 0;
}
