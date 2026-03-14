# Stage 1: Build the rootfs with Chromium and security tooling.
FROM alpine:3.23 AS rootfs-builder

RUN mkdir /rootfs && \
    apk add --no-cache --root /rootfs --initdb \
        alpine-base busybox chromium font-noto mesa-dri-gallium \
        xvfb-run dbus iptables iproute2 e2fsprogs util-linux \
        rng-tools haveged && \
    rm -rf /rootfs/var/cache/apk/*

# Create unprivileged users.
RUN echo 'secwatch:x:999:999:Security Monitor:/:/sbin/nologin' >> /rootfs/etc/passwd && \
    echo 'secwatch:x:999:' >> /rootfs/etc/group && \
    echo 'browser:x:1000:1000:Browser User:/home/browser:/bin/sh' >> /rootfs/etc/passwd && \
    echo 'browser:x:1000:' >> /rootfs/etc/group && \
    mkdir -p /rootfs/home/browser && \
    chown 1000:1000 /rootfs/home/browser

# Copy VM scripts.
COPY alpine/browserinit /rootfs/etc/browserinit
COPY alpine/browserrouter.sh /rootfs/sbin/browserrouter.sh
COPY alpine/sysctl.conf /rootfs/etc/sysctl.conf
RUN chmod 755 /rootfs/etc/browserinit /rootfs/sbin/browserrouter.sh && \
    ln -sf /etc/browserinit /rootfs/init

# Copy honey token templates.
COPY alpine/honeytokens/ /rootfs/etc/honeytokens/

# Create seccomp directory.
RUN mkdir -p /rootfs/etc/seccomp

# Stage 2: Compile seccomp loader and security monitor daemon.
FROM alpine:3.23 AS seccomp-builder

RUN apk add --no-cache gcc musl-dev linux-headers libseccomp-dev make

COPY alpine/seccomp/seccomp_loader.c /build/seccomp_loader.c
COPY alpine/secwatchd/ /build/secwatchd/

# Build seccomp_loader (applies BPF filter then execs target binary).
RUN gcc -static -O2 -Wall -o /build/seccomp_loader /build/seccomp_loader.c

# Build chromium seccomp filter as raw BPF bytecode.
COPY alpine/seccomp/chromium_filter.c /build/chromium_filter.c
RUN gcc -O2 -Wall -o /build/gen_filter /build/chromium_filter.c -lseccomp && \
    /build/gen_filter > /build/chromium.bpf

# Build secwatchd (security monitor daemon).
RUN cd /build/secwatchd && make

# Stage 3: Extract kernel.
FROM alpine:3.23 AS kernel

RUN apk add --no-cache linux-virt
RUN cp /boot/vmlinuz-* /vmlinuz

# Stage 4: Assemble final output.
FROM alpine:3.23 AS output

RUN apk add --no-cache cpio e2fsprogs

COPY --from=rootfs-builder /rootfs /rootfs
COPY --from=seccomp-builder /build/seccomp_loader /rootfs/usr/local/bin/seccomp_loader
COPY --from=seccomp-builder /build/chromium.bpf /rootfs/etc/seccomp/chromium.bpf
COPY --from=seccomp-builder /build/secwatchd/secwatchd /rootfs/usr/local/bin/secwatchd
COPY --from=kernel /vmlinuz /vmlinuz

RUN chmod 755 /rootfs/usr/local/bin/seccomp_loader /rootfs/usr/local/bin/secwatchd && \
    chmod 644 /rootfs/etc/seccomp/chromium.bpf

# Create initramfs.
RUN cd /rootfs && find . | cpio -o -H newc | gzip -9 > /initramfs.gz

# Create 128MB ext4 state disk (browser profile persistence).
RUN dd if=/dev/zero of=/state.img bs=1M count=128 2>/dev/null && \
    mkfs.ext4 -F -L browserstate -q \
        -E lazy_itable_init=0,lazy_journal_init=0 /state.img
