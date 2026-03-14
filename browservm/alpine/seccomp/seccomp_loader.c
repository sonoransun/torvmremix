/*
 * seccomp_loader.c — Load a pre-compiled BPF seccomp filter from a file,
 * apply it to the current process, then exec the target binary.
 *
 * Usage: seccomp_loader /path/to/target [args...]
 *
 * The BPF filter is read from /etc/seccomp/chromium.bpf.
 */

#include <errno.h>
#include <fcntl.h>
#include <linux/filter.h>
#include <linux/seccomp.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <sys/prctl.h>
#include <sys/stat.h>
#include <unistd.h>

#define BPF_FILTER_PATH "/etc/seccomp/chromium.bpf"
#define MAX_FILTER_SIZE (4096 * sizeof(struct sock_filter))

int main(int argc, char *argv[]) {
    if (argc < 2) {
        fprintf(stderr, "usage: seccomp_loader <program> [args...]\n");
        return 1;
    }

    /* Read BPF filter from file. */
    int fd = open(BPF_FILTER_PATH, O_RDONLY | O_CLOEXEC);
    if (fd < 0) {
        fprintf(stderr, "seccomp_loader: open %s: %s\n",
                BPF_FILTER_PATH, strerror(errno));
        return 1;
    }

    struct stat st;
    if (fstat(fd, &st) < 0) {
        perror("seccomp_loader: fstat");
        close(fd);
        return 1;
    }

    if (st.st_size == 0 || (size_t)st.st_size > MAX_FILTER_SIZE) {
        fprintf(stderr, "seccomp_loader: filter size %ld invalid\n",
                (long)st.st_size);
        close(fd);
        return 1;
    }

    if (st.st_size % sizeof(struct sock_filter) != 0) {
        fprintf(stderr, "seccomp_loader: filter size not aligned to "
                "sock_filter (%zu bytes)\n", sizeof(struct sock_filter));
        close(fd);
        return 1;
    }

    struct sock_filter *filter = malloc(st.st_size);
    if (!filter) {
        perror("seccomp_loader: malloc");
        close(fd);
        return 1;
    }

    ssize_t n = read(fd, filter, st.st_size);
    close(fd);
    if (n != st.st_size) {
        fprintf(stderr, "seccomp_loader: short read: %zd/%ld\n",
                n, (long)st.st_size);
        free(filter);
        return 1;
    }

    unsigned short count = (unsigned short)(st.st_size / sizeof(struct sock_filter));

    /* Prevent privilege escalation via execve. */
    if (prctl(PR_SET_NO_NEW_PRIVS, 1, 0, 0, 0) < 0) {
        perror("seccomp_loader: PR_SET_NO_NEW_PRIVS");
        free(filter);
        return 1;
    }

    /* Apply the BPF filter. */
    struct sock_fprog prog = {
        .len = count,
        .filter = filter,
    };

    if (prctl(PR_SET_SECCOMP, SECCOMP_MODE_FILTER, &prog) < 0) {
        perror("seccomp_loader: PR_SET_SECCOMP");
        free(filter);
        return 1;
    }

    free(filter);

    /* Exec the target program. */
    execvp(argv[1], &argv[1]);
    fprintf(stderr, "seccomp_loader: exec %s: %s\n",
            argv[1], strerror(errno));
    return 1;
}
