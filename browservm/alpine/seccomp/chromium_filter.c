/*
 * chromium_filter.c — Generate a seccomp-bpf allowlist filter for Chromium
 * and write the raw BPF bytecode to stdout.
 *
 * Compiled and run at Docker build time:
 *   gcc -o gen_filter chromium_filter.c -lseccomp
 *   ./gen_filter > chromium.bpf
 */

#include <seccomp.h>
#include <stdio.h>
#include <stdlib.h>
#include <unistd.h>

int main(void) {
    /* Default action: kill the process on disallowed syscall. */
    scmp_filter_ctx ctx = seccomp_init(SCMP_ACT_KILL_PROCESS);
    if (!ctx) {
        fprintf(stderr, "seccomp_init failed\n");
        return 1;
    }

    /* Allowlist: syscalls Chromium needs to function.
     * This is a conservative list based on analysis of Chromium's
     * Linux sandbox requirements with --no-sandbox mode. */
    int allowed[] = {
        /* File I/O */
        SCMP_SYS(read), SCMP_SYS(write), SCMP_SYS(open), SCMP_SYS(openat),
        SCMP_SYS(close), SCMP_SYS(fstat), SCMP_SYS(newfstatat), SCMP_SYS(statx),
        SCMP_SYS(lstat), SCMP_SYS(stat), SCMP_SYS(lseek),
        SCMP_SYS(access), SCMP_SYS(faccessat), SCMP_SYS(faccessat2),
        SCMP_SYS(readlink), SCMP_SYS(readlinkat),
        SCMP_SYS(getdents), SCMP_SYS(getdents64),
        SCMP_SYS(getcwd), SCMP_SYS(chdir), SCMP_SYS(fchdir),
        SCMP_SYS(rename), SCMP_SYS(renameat), SCMP_SYS(renameat2),
        SCMP_SYS(mkdir), SCMP_SYS(mkdirat), SCMP_SYS(rmdir),
        SCMP_SYS(creat), SCMP_SYS(link), SCMP_SYS(unlink), SCMP_SYS(unlinkat),
        SCMP_SYS(symlink), SCMP_SYS(symlinkat),
        SCMP_SYS(chmod), SCMP_SYS(fchmod), SCMP_SYS(fchmodat),
        SCMP_SYS(chown), SCMP_SYS(fchown), SCMP_SYS(fchownat), SCMP_SYS(lchown),
        SCMP_SYS(truncate), SCMP_SYS(ftruncate),
        SCMP_SYS(fcntl), SCMP_SYS(flock),
        SCMP_SYS(fsync), SCMP_SYS(fdatasync),
        SCMP_SYS(umask), SCMP_SYS(statfs), SCMP_SYS(fstatfs),
        SCMP_SYS(sendfile),
        SCMP_SYS(dup), SCMP_SYS(dup2), SCMP_SYS(dup3),
        SCMP_SYS(pipe), SCMP_SYS(pipe2),

        /* Memory management */
        SCMP_SYS(mmap), SCMP_SYS(mprotect), SCMP_SYS(munmap),
        SCMP_SYS(brk), SCMP_SYS(mremap), SCMP_SYS(msync),
        SCMP_SYS(mincore), SCMP_SYS(madvise),
        SCMP_SYS(mlock), SCMP_SYS(mlock2), SCMP_SYS(munlock),
        SCMP_SYS(memfd_create), SCMP_SYS(membarrier),

        /* Process management */
        SCMP_SYS(clone), SCMP_SYS(clone3), SCMP_SYS(fork), SCMP_SYS(vfork),
        SCMP_SYS(execve), SCMP_SYS(execveat),
        SCMP_SYS(exit), SCMP_SYS(exit_group),
        SCMP_SYS(wait4), SCMP_SYS(waitid),
        SCMP_SYS(kill), SCMP_SYS(tgkill),
        SCMP_SYS(getpid), SCMP_SYS(gettid), SCMP_SYS(getppid),
        SCMP_SYS(getuid), SCMP_SYS(getgid), SCMP_SYS(geteuid), SCMP_SYS(getegid),
        SCMP_SYS(setpgid), SCMP_SYS(getpgrp), SCMP_SYS(setsid),
        SCMP_SYS(setreuid), SCMP_SYS(setregid),
        SCMP_SYS(getgroups), SCMP_SYS(setgroups),
        SCMP_SYS(setresuid), SCMP_SYS(setresgid),
        SCMP_SYS(getresuid), SCMP_SYS(getresgid),
        SCMP_SYS(getrlimit), SCMP_SYS(prlimit64), SCMP_SYS(getrusage),
        SCMP_SYS(sysinfo), SCMP_SYS(times), SCMP_SYS(uname),
        SCMP_SYS(prctl), SCMP_SYS(arch_prctl),
        SCMP_SYS(set_tid_address), SCMP_SYS(set_robust_list),
        SCMP_SYS(get_robust_list),
        SCMP_SYS(sched_yield), SCMP_SYS(sched_getaffinity),
        SCMP_SYS(sched_setaffinity),
        SCMP_SYS(close_range), SCMP_SYS(rseq),

        /* Signals */
        SCMP_SYS(rt_sigaction), SCMP_SYS(rt_sigprocmask),
        SCMP_SYS(rt_sigreturn), SCMP_SYS(rt_sigpending),
        SCMP_SYS(rt_sigtimedwait), SCMP_SYS(rt_sigsuspend),
        SCMP_SYS(sigaltstack), SCMP_SYS(restart_syscall),
        SCMP_SYS(signalfd), SCMP_SYS(signalfd4),

        /* Timers and clocks */
        SCMP_SYS(nanosleep), SCMP_SYS(clock_nanosleep),
        SCMP_SYS(clock_gettime), SCMP_SYS(clock_getres),
        SCMP_SYS(gettimeofday),
        SCMP_SYS(getitimer), SCMP_SYS(setitimer), SCMP_SYS(alarm),
        SCMP_SYS(timerfd_create), SCMP_SYS(timerfd_settime),
        SCMP_SYS(timerfd_gettime),

        /* Networking (SOCKS5 proxy connection) */
        SCMP_SYS(socket), SCMP_SYS(connect), SCMP_SYS(accept), SCMP_SYS(accept4),
        SCMP_SYS(bind), SCMP_SYS(listen),
        SCMP_SYS(sendto), SCMP_SYS(recvfrom),
        SCMP_SYS(sendmsg), SCMP_SYS(recvmsg),
        SCMP_SYS(shutdown),
        SCMP_SYS(getsockname), SCMP_SYS(getpeername),
        SCMP_SYS(socketpair),
        SCMP_SYS(setsockopt), SCMP_SYS(getsockopt),

        /* Shared memory (Chromium IPC) */
        SCMP_SYS(shmget), SCMP_SYS(shmat), SCMP_SYS(shmctl), SCMP_SYS(shmdt),

        /* Event polling */
        SCMP_SYS(select), SCMP_SYS(pselect6),
        SCMP_SYS(poll), SCMP_SYS(ppoll),
        SCMP_SYS(epoll_create), SCMP_SYS(epoll_create1),
        SCMP_SYS(epoll_ctl), SCMP_SYS(epoll_wait), SCMP_SYS(epoll_pwait),
        SCMP_SYS(eventfd), SCMP_SYS(eventfd2),

        /* inotify (Chromium file watching) */
        SCMP_SYS(inotify_init), SCMP_SYS(inotify_init1),
        SCMP_SYS(inotify_add_watch), SCMP_SYS(inotify_rm_watch),

        /* Futex and threading */
        SCMP_SYS(futex),

        /* I/O control */
        SCMP_SYS(ioctl),

        /* Entropy */
        SCMP_SYS(getrandom),
    };

    int n = sizeof(allowed) / sizeof(allowed[0]);
    for (int i = 0; i < n; i++) {
        if (seccomp_rule_add(ctx, SCMP_ACT_ALLOW, allowed[i], 0) < 0) {
            /* Some syscalls may not exist on all architectures; ignore. */
        }
    }

    /* Export raw BPF bytecode to stdout. */
    int fd = fileno(stdout);
    if (seccomp_export_bpf(ctx, fd) < 0) {
        fprintf(stderr, "seccomp_export_bpf failed\n");
        seccomp_release(ctx);
        return 1;
    }

    seccomp_release(ctx);
    return 0;
}
