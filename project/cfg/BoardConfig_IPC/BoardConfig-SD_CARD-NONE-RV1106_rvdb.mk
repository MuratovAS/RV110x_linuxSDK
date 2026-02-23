#!/bin/bash

# Target arch
export RK_ARCH=arm

# Target CHIP
export RK_CHIP=rv1106

# Target rootfs: buildroot/busybox
export RK_TARGET_ROOTFS=buildroot

# Target Toolchain Cross Compile
export RK_TOOLCHAIN_CROSS=arm-rockchip830-linux-uclibcgnueabihf

# Target boot medium: emmc/spi_nor/spi_nand
export RK_BOOT_MEDIUM=sd_card

# Uboot defconfig
export RK_UBOOT_DEFCONFIG=rv1106_defconfig

# Uboot defconfig fragment
export RK_UBOOT_DEFCONFIG_FRAGMENT=rk-sfc.config

# Kernel defconfig
export RK_KERNEL_DEFCONFIG=rv1106_rvdb_defconfig

# Kernel dts
export RK_KERNEL_DTS=rv1106g-rvdb.dts

# buildroot defconfig
export RK_BUILDROOT_DEFCONFIG=rv1106_rvdb_defconfig

#misc image
export RK_MISC=wipe_all-misc.img

# Config CMA size in environment
export RK_BOOTARGS_CMA_SIZE="48M"

# config partition in environment
# RK_PARTITION_CMD_IN_ENV format:
#     <partdef>[,<partdef>]
#       <partdef> := <size>[@<offset>](part-name)
# Note:
#   If the first partition offset is not 0x0, it must be added. Otherwise, it needn't adding.
# export RK_PARTITION_CMD_IN_ENV="256K(env),1M@256K(idblock),1M(uboot),5M(boot),160M(rootfs),48M(oem),32M(userdata)"
export RK_PARTITION_CMD_IN_ENV="32K(env),512K@32K(idblock),256K(uboot),32M(boot),2G(rootfs)"

# config partition's filesystem type (squashfs is readonly)
# emmc:    squashfs/ext4
# nand:    squashfs/ubifs
# spi nor: squashfs/jffs2
# RK_PARTITION_FS_TYPE_CFG format:
#     AAAA:/BBBB/CCCC@ext4
#         AAAA ----------> partition name
#         /BBBB/CCCC ----> partition mount point
#         ext4 ----------> partition filesystem type
# export RK_PARTITION_FS_TYPE_CFG=rootfs@IGNORE@ubifs,oem@/oem@ubifs,userdata@/userdata@ubifs
export RK_PARTITION_FS_TYPE_CFG=rootfs@IGNORE@ext4

# enable install app to oem partition
export RK_BUILD_APP_TO_OEM_PARTITION=n
