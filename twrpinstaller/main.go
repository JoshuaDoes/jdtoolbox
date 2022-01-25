package main

import (
	//"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"runtime"
	"strings"

	flag "github.com/spf13/pflag"
)

var (
	wd, mb string
	twrp, boot, recovery string
)

func init() {
	flag.StringVar(&wd, "wd", "/tmp/", "path to tmp directory for process")
	flag.StringVar(&mb, "magiskboot", "/data/adb/magisk/magiskboot", "path to magiskboot for repacking")
	flag.StringVar(&twrp, "twrp", "", "path to twrp to install")
	flag.StringVar(&boot, "boot", "", "path to boot partition to repack, ignored with recovery")
	flag.StringVar(&recovery, "recovery", "", "path to recovery partition to flash, invalidating boot repacking")
	flag.Parse()

	if _, err := os.Stat(wd); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	if _, err := os.Stat(mb); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	if _, err := os.Stat(twrp); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	if boot != "" {
		if _, err := os.Stat(boot); err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	}
	if recovery != "" {
		if _, err := os.Stat(recovery); err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	}
}

func log(msg string) {
	if msg == "" {
		fmt.Print("\n")
	} else {
		fmt.Println("  • " + msg)
	}
}
func check(err error) {
	if err != nil && err != io.EOF {
		fmt.Println(err)
		os.Exit(1)
	}
}

func main() {
	typeTWRP := file(twrp)
	if !strings.Contains(typeTWRP, "Android bootimg") {
		check(fmt.Errorf("[%s] is not an Android bootimg: %s", twrp, typeTWRP))
	}
	log("Successfully validated TWRP as " + typeTWRP)

	if recovery != "" {
		twrpRecovery()
	} else if boot != "" {
		twrpBoot()
	} else {
		check(fmt.Errorf("What are we supposed to do, exactly?"))
	}
}

func twrpBoot() {
	log("Backing up boot to [/sdcard/boot.img]...")
	check(cp(boot, "/sdcard/boot.img"))

	typeBoot := file("/sdcard/boot.img")
	if !strings.Contains(typeBoot, "Android bootimg") {
		check(fmt.Errorf("[%s] is not an Android bootimg: %s", boot, typeBoot))
	}
	log("Successfully validated boot as " + typeBoot)

	log("Unpacking boot to [" + wd+"boot]...")
	check(os.MkdirAll(wd+"boot", 0644))
	check(run(mb, wd+"boot", "unpack", "-h", "-n", boot))

	log("Unpacking TWRP to [" + wd+"twrp]...")
	check(os.MkdirAll(wd+"twrp", 0644))
	check(run(mb, wd+"twrp", "unpack", "-h", "-n", twrp))

	log("Decompressing TWRP ramdisk...")
	check(run(mb, wd+"twrp", "decompress", wd+"twrp/ramdisk.cpio", wd+"twrp/ramdiskdecomp.cpio"))

	log("Replacing boot ramdisk with decompressed TWRP ramdisk...")
	check(cp(wd+"twrp/ramdiskdecomp.cpio", wd+"boot/ramdisk.cpio"))

	log("Patching boot ramdisk...")
	check(run(mb, wd+"boot", "cpio", wd+"boot/ramdisk.cpio", "patch"))

	log("Repacking boot...")
	check(run(mb, wd+"boot", "repack", boot, "new.img"))

	log("Flashing boot...")
	check(cp(wd+"boot/new.img", boot))
}

func twrpRecovery() {
	log("Backing up recovery to [/sdcard/recovery.img]...")
	check(cp(recovery, "/sdcard/recovery.img"))

	log("Flashing recovery...")
	check(cp(twrp, recovery))
}

func run(prog, dir string, args ...string) error {
	cmd := exec.Command(prog, args...)
	//cmd.Stdout = os.Stdout
	cmd.Stdout = os.NewFile(0, os.DevNull)
	//cmd.Stderr = os.Stderr
	cmd.Stderr = os.NewFile(0, os.DevNull)
	cmd.Stdin = os.Stdin
	cmd.Dir = dir
	return cmd.Run()
}

func cp(src, dst string) error {
	data, err := ioutil.ReadFile(src)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(dst, data, 0644)
}

func file(path string) string {
	fileType, err := exec.Command(wd+"bin/file-" + runtime.GOARCH, path).Output()
	if err != nil {
		return fmt.Sprintf("invalid: %v", err)
	}

	return string(fileType[len(path)+2:len(fileType)-1])
}
