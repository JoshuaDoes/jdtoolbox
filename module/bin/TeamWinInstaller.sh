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

    exit 1
}

on_done() {
    echo
    echo
    echo "$P TWRP installed!"
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

Installing TWRP with twrpinstaller by JoshuaDoes
Based on kramflash by kdrag0n
Based on twrpRepacker by TeamWin Recovery Project

---------------------------------

EOF

twrp_image=$1
echo "$P TWRP image: $twrp_image"
boot_slot="$(cat /proc/cmdline | tr ' ' '\n' | grep androidboot.slot_suffix | sed 's/.*=_\(.*\)/\1/')"
[[ ! -z "$boot_slot" ]] && export boot_slot="_$boot_slot" && echo "$P Boot slot: $boot_slot" || echo "$P Boot has no secondary slots"
echo "$P Scanning for boot partition, please wait..."
boot_part="$(find_part_by_name boot$boot_slot)"
[[ ! -z "$boot_part" ]] && echo "$P Boot partition: $boot_part" || (echo "$P No boot partition found" && exit 1) #How the hell don't you have boot?
echo "$P Scanning for recovery partition, please wait..."
recovery_part="$(find_part_by_name recovery)"
[[ ! -z "$recovery_part" ]] && echo "$P Recovery partition: $recovery_part" || echo "$P No recovery partition found, ignoring..."
echo

part_args="--boot $boot_part"
[[ ! -z "$recovery_part" ]] && export part_args="--recovery $recovery_part"
./bin/twrpinstaller --wd "/data/local/tmp/" --magiskboot "$MAGISKBOOT" --twrp "$twrp_image" $part_args

sync

trap on_done EXIT
exit 0
