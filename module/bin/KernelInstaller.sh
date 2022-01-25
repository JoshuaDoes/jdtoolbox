#!/bin/sh

source /data/adb/magisk/util_functions.sh

P="  •"

on_error() {
    echo
    echo
    echo "$P ERROR!"
    echo
    echo
    echo
    echo
    echo
    
    ls -la /dev/tmp/*

    exit 1
}

on_done() {
    echo
    echo
    echo "$P Kernel installed!"
    echo
    echo
    echo
    echo
    echo

    exit 0
}

trap on_error EXIT
set -e

find_part_by_name() {
    #echo "/dev/block/by-name/$1"
    echo $(find_block $1)
}

MAGISKBOOT=/data/adb/magisk/magiskboot
(ls $MAGISKBOOT >> /dev/null 2>&1 && echo "$P Found magiskboot: $MAGISKBOOT") || (echo "$P Missing dependency magiskboot" && exit 1)

cat <<EOF

Installing kernel with kernelinstaller by JoshuaDoes
Based on kramflash by kdrag0n

---------------------------------

EOF

kernel_image=$1
echo "$P Kernel image: $kernel_image"
boot_slot="$(cat /proc/cmdline | tr ' ' '\n' | grep androidboot.slot_suffix | sed 's/.*=_\(.*\)/\1/')"
echo "$P Boot slot: $boot_slot"
boot_part="$(find_part_by_name boot_$boot_slot)"
echo "$P Boot partition: $boot_part"
vendor_boot_part="$(find_part_by_name vendor_boot_$boot_slot)"
echo "$P Vendor boot partition: $vendor_boot_part"
echo

./bin/krnlinst --wd "$TMPDIR/" --magiskboot "$MAGISKBOOT" --kernel "$kernel_image" --boot "$boot_part" --vendorboot "$vendor_boot_part"

#echo "$P Unpacking images..."
#mkdir -p /data/local/tmp/boot_$boot_slot /data/local/tmp/vendor_boot_$boot_slot
#cd /data/local/tmp/boot_$boot_slot
#boot_info="$($MAGISKBOOT unpack -n "$boot_part" 2>&1)"
#echo "$P $boot_info"
#boot_ver="$(echo "$boot_info" | grep HEADER_VER | awk '{print $2}' | tr -d '[]')"
#echo "$P Detected boot v$boot_ver"
#cd ../vendor_boot_$boot_slot
#vendor_boot_info="$($MAGISKBOOT unpack -n "$vendor_boot_part" 2>&1)"
#echo "$P $vendor_boot_info"
#echo

#cd ..
#cp "$kernel_image" boot_$boot_slot/kernel
#echo "$P Injected kernel into boot_$boot_slot"
#(ls "$kernel_dtb" >> /dev/null 2>&1 && cp "$kernel_dtb" boot_$boot_slot/dtb && echo "$P Injected dtb into boot_$boot_slot" && cp "$kernel_dtb" vendor_boot_$boot_slot/dtb && echo "$P Injected dtb into vendor_boot_$boot_slot")

#cd boot_$boot_slot
#$MAGISKBOOT repack -n "$boot_part" new.img 2>/dev/null
#(ls new.img >> /dev/null 2>&1 && echo "$P Repacked boot_$boot_slot") || (echo "$P Missing repacked boot_$boot_slot" && exit 1)
#(ls "$kernel_dtb" >> /dev/null 2>&1 && cd ../vendor_boot_$boot_slot && $MAGISKBOOT repack -n "$vendor_boot_part" new.img 2>/dev/null && (ls new.img >> /dev/null 2>&1 && echo "$P Repacked vendor_boot_$boot_slot") || (echo "$P Missing repacked vendor_boot_$boot_slot" && exit 1))

#cd ..
#echo "$P Flashing images..."
#cat /data/local/tmp/boot_$boot_slot/new.img > "$boot_part"
#echo "$P Flashed boot_$boot_slot"
#cat /data/local/tmp/vendor_boot_$boot_slot/new.img > "$vendor_boot_part"
#echo "$P Flashed vendor_boot_$boot_slot"
sync

trap on_done EXIT
exit 0
