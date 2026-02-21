# Multistage Docker build for TorVM Alpine-based VM image
# Produces: vmlinuz, initramfs.gz, state.img

# ============================================================
# Stage 1: Build the root filesystem
# ============================================================
FROM alpine:3.21 AS rootfs-builder

# Install packages into a separate rootfs directory
RUN mkdir -p /rootfs && \
    apk add --no-cache --initdb --root /rootfs \
        alpine-base \
        busybox \
        openrc \
        tor \
        iptables \
        iproute2 \
        e2fsprogs \
        util-linux && \
    # Clean up apk cache in rootfs
    rm -rf /rootfs/var/cache/apk/*

# Create tor user/group in rootfs (Alpine's tor package creates these,
# but ensure they exist with correct IDs)
RUN grep -q '^tor:' /rootfs/etc/passwd || \
    echo 'tor:x:52:52:tor:/home/tor:/bin/false' >> /rootfs/etc/passwd && \
    grep -q '^tor:' /rootfs/etc/group || \
    echo 'tor:x:52:' >> /rootfs/etc/group

# Create required directories
RUN mkdir -p /rootfs/etc/tor \
             /rootfs/var/log/tor \
             /rootfs/var/run/tor \
             /rootfs/home/tor/data \
             /rootfs/sbin \
             /rootfs/bin \
             /rootfs/home

# Copy configuration files
COPY alpine/torrc /rootfs/etc/tor/torrc
COPY alpine/vmrouter.sh /rootfs/sbin/vmrouter.sh
COPY alpine/torvminit /rootfs/etc/torvminit
COPY alpine/entropy-update.sh /rootfs/bin/entropy-update.sh
COPY alpine/sysctl.conf /rootfs/etc/sysctl.conf

# Set permissions
RUN chmod 644 /rootfs/etc/tor/torrc && \
    chmod 755 /rootfs/sbin/vmrouter.sh && \
    chmod 755 /rootfs/etc/torvminit && \
    chmod 755 /rootfs/bin/entropy-update.sh && \
    chmod 644 /rootfs/etc/sysctl.conf && \
    chown -R 52:52 /rootfs/home/tor && \
    chmod 700 /rootfs/home/tor/data && \
    chown 52:52 /rootfs/var/log/tor && \
    chown 52:52 /rootfs/var/run/tor

# Set torvminit as the init process
RUN ln -sf /etc/torvminit /rootfs/init

# ============================================================
# Stage 2: Extract the kernel
# ============================================================
FROM alpine:3.21 AS kernel

RUN apk add --no-cache linux-virt

# ============================================================
# Stage 3: Assemble initramfs and state disk
# ============================================================
FROM alpine:3.21 AS assembler

RUN apk add --no-cache cpio gzip e2fsprogs coreutils

# Copy rootfs from stage 1
COPY --from=rootfs-builder /rootfs /rootfs

# Copy kernel from stage 2
COPY --from=kernel /boot/vmlinuz-* /out/vmlinuz

# Create initramfs from rootfs
RUN cd /rootfs && \
    find . | cpio -o -H newc 2>/dev/null | gzip -9 > /out/initramfs.gz

# Create 64MB ext4 state image for persistent Tor data
RUN dd if=/dev/zero of=/out/state.img bs=1M count=64 2>/dev/null && \
    mkfs.ext4 -F -L torstate -q /out/state.img

# ============================================================
# Stage 4: Output artifacts
# ============================================================
FROM scratch AS output
COPY --from=assembler /out/vmlinuz /vmlinuz
COPY --from=assembler /out/initramfs.gz /initramfs.gz
COPY --from=assembler /out/state.img /state.img
