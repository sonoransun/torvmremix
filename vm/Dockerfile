# Multistage Docker build for TorVM Alpine-based VM image
# Produces: vmlinuz, initramfs.gz, state.img

# ============================================================
# Stage 1: Build the root filesystem
# ============================================================
FROM alpine:3.23 AS rootfs-builder

# Install packages and set up directories/permissions in a single layer
RUN mkdir -p /rootfs && \
    apk add --no-cache --initdb --root /rootfs \
        alpine-base \
        busybox \
        openrc \
        tor \
        iptables \
        iproute2 \
        e2fsprogs \
        util-linux \
        rng-tools && \
    rm -rf /rootfs/var/cache/apk/* && \
    mkdir -p /rootfs/etc/tor \
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

# Set permissions and init symlink
RUN chmod 644 /rootfs/etc/tor/torrc && \
    chmod 755 /rootfs/sbin/vmrouter.sh && \
    chmod 755 /rootfs/etc/torvminit && \
    chmod 755 /rootfs/bin/entropy-update.sh && \
    chmod 644 /rootfs/etc/sysctl.conf && \
    chown -R 52:52 /rootfs/home/tor && \
    chmod 700 /rootfs/home/tor/data && \
    chown 52:52 /rootfs/var/log/tor && \
    chown 52:52 /rootfs/var/run/tor && \
    ln -sf /etc/torvminit /rootfs/init

# ============================================================
# Stage 2: Extract the kernel
# ============================================================
FROM alpine:3.23 AS kernel

RUN apk add --no-cache linux-virt

# ============================================================
# Stage 3: Assemble initramfs and state disk (final output)
# ============================================================
FROM alpine:3.23 AS output

RUN apk add --no-cache cpio gzip e2fsprogs

# Copy rootfs from stage 1
COPY --from=rootfs-builder /rootfs /rootfs

# Copy kernel from stage 2
COPY --from=kernel /boot/vmlinuz-* /vmlinuz

# Create initramfs from rootfs
RUN cd /rootfs && \
    find . | cpio -o -H newc 2>/dev/null | gzip -6 > /initramfs.gz

# Create 64MB ext4 state image for persistent Tor data
RUN truncate -s 64M /state.img && \
    mkfs.ext4 -F -L torstate -q /state.img
