package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	flag "github.com/spf13/pflag"
)

var (
	wd, mb string
	kernel, dtb string
	boot, vendorboot string
)

func init() {
	flag.StringVar(&wd, "wd", "/tmp/", "path to tmp directory for process")
	flag.StringVar(&mb, "magiskboot", "/data/adb/magisk/magiskboot", "path to magiskboot for repacking")
	flag.StringVar(&kernel, "kernel", "", "path to kernel to install")
	flag.StringVar(&dtb, "dtb", "", "path to dtb to install")
	flag.StringVar(&boot, "boot", "", "path to boot partition to modify")
	flag.StringVar(&vendorboot, "vendorboot", "", "path to vendor boot partition to modify")
	flag.Parse()

	if _, err := os.Stat(wd); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	if _, err := os.Stat(mb); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	if _, err := os.Stat(kernel); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	if _, err := os.Stat(boot); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	if _, err := os.Stat(vendorboot); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func log(msg string) {
	fmt.Println("  • " + msg)
}
func check(err error) {
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func main() {
	/*TODO:
	- Detect if kernel is an archive (ZIP, RAR, 7Z), and if so, unpack it and search for a kernel and dtb pair
	- Detect if kernel is an Android boot image, and if so, unpack the ramdisk and search for a kernel and dtb pair,
		otherwise use the kernel and dtb from the boot image
	- Detect if kernel is not a kernel and error out
	*/
	
	log("Backing up boot to [/sdcard/boot.img]...")
	check(cp(boot, "/sdcard/boot.img"))

	typeBoot := file("/sdcard/boot.img")
	if !strings.Contains(typeBoot, "Android bootimg") {
		check(fmt.Errorf("[%s] is not an Android bootimg: %s", boot, typeBoot))
	}
	log("Successfully validated boot as " + typeBoot)

	if vendorboot != "" {
		log("Backing up vendor boot to [/sdcard/vendor_boot.img]...")
		check(cp(vendorboot, "/sdcard/vendor_boot.img"))

		typeVendorBoot := file("/sdcard/vendor_boot.img")
		if !strings.Contains(typeVendorBoot, "data") {
			check(fmt.Errorf("[%s] is not correctly identified as data: %s", vendorboot, typeVendorBoot))
		}
		log("Successfully validated vendor boot as " + typeVendorBoot)
	}

	typeKernel := file(kernel)
	if strings.Contains(typeKernel, "Linux kernel") {
		//TODO: Check for architecture compatibility
		typeKernel = "linux"
	} else if strings.Contains(typeKernel, "Android bootimg") {
		typeKernel = "boot"
	} else {
		check(fmt.Errorf("[%s] is not an Android kernel: %s", kernel, typeKernel))
	}
	log("Successfully validated kernel image as " + file(kernel))
	
	if typeKernel == "linux" {
		log("Nothing to do for kernel")
	} else if typeKernel == "boot" {
		log("Unpacking kernel image to [" + wd+"kernel]...")
		check(os.MkdirAll(wd+"kernel/rd", 0644))
		check(magiskboot(wd+"kernel", "unpack", "-h", kernel))

		log("Unpacking kernel image ramdisk to [" + wd+"kernel/rd]...")
		check(magiskboot(wd+"kernel/rd", "cpio", wd+"kernel/ramdisk.cpio", "extract"))

		log("Walking for kernel and dtb...")
		kernel = ""
		dtb = ""
		err := filepath.Walk(wd+"kernel/rd", func(path string, info os.FileInfo, err error) error {
			typeFile := file(path)
			if strings.Contains(typeFile, "compressed data") {
				check(magiskboot(wd+"kernel", "decompress", path, path+".decompressed"))
				check(os.RemoveAll(path))
				path += ".decompressed"
				typeFile = file(path)
			}
			if strings.Contains(typeFile, "Linux kernel") {
				if kernel != "" {
					check(fmt.Errorf("TODO: kernel choice not yet supported"))
				}
				kernel = path
				log("Found kernel: " + path)
			}
			if strings.Contains(typeFile, "Device Tree Blob") {
				if dtb != "" {
					check(fmt.Errorf("TODO: dtb choice not yet supported"))
				}
				dtb = path
				log("Found dtb: " + path)
			}
			if kernel != "" && dtb != "" {
				return io.EOF
			}
			return nil
		})
		if err != nil && err != io.EOF {
			check(fmt.Errorf("walking ramdisk failed: %v", err))
		}

		if kernel == "" && dtb != "" {
			check(fmt.Errorf("finding kernel failed but found dtb, bailing"))
		}
		if kernel == "" && dtb == "" {
			log("No kernel in ramdisk, using kernel from selected boot image")
			kernel = wd+"kernel/kernel"
			dtb = wd+"kernel/dtb"
		}
			
		typeKernel = file(kernel)
		if !strings.Contains(typeKernel, "Linux kernel") {
			check(fmt.Errorf("[%s] is not an Android kernel: %s", kernel, typeKernel))
		}
		log("Successfully validated kernel as " + typeKernel)

		log("Cleaning up unpacked kernel image...")
		check(cp(kernel, wd+"kernel.tmp"))
		check(cp(dtb, wd+"dtb.tmp"))
		kernel = wd+"kernel.tmp"
		dtb = wd+"dtb.tmp"
		check(os.RemoveAll(wd+"kernel"))
	} else {
		check(fmt.Errorf("What are we supposed to do, exactly?"))
	}
	
	if dtb == "" {
		check(fmt.Errorf("TODO: embedded dtbs and dtb scanning not yet supported"))
	}
	typeDTB := file(dtb)
	if !strings.Contains(typeDTB, "Device Tree Blob") {
		check(fmt.Errorf("[%s] is not a Device Tree Blob: %s", dtb, typeDTB))
	}
	log("Successfully validated dtb as " + typeDTB)

	log("Unpacking boot to [" + wd+"boot]...")
	check(os.MkdirAll(wd+"boot", 0644))
	check(magiskboot(wd+"boot", "unpack", "-n", boot))

	log("Injecting kernel into boot...")
	check(cp(kernel, wd+"boot/kernel"))
	log("Injecting dtb into boot...")
	check(cp(dtb, wd+"boot/dtb"))

	log("Repacking boot...")
	check(magiskboot(wd+"boot", "repack", "-n", boot, wd+"boot.img"))
	log("Cleaning up unpacked boot...")
	check(os.RemoveAll(wd+"boot"))

	log("Flashing boot...")
	check(cp(wd+"boot.img", boot))

	if vendorboot != "" {
		log("Unpacking vendor boot to [" + wd+"vendor_boot]...")
		check(os.MkdirAll(wd+"vendor_boot", 0644))
		check(magiskboot(wd+"vendor_boot", "unpack", "-n", vendorboot))

		log("Injecting dtb into vendor boot...")
		check(cp(dtb, wd+"vendor_boot/dtb"))

		log("Repacking vendor boot...")
		check(magiskboot(wd+"vendor_boot", "repack", "-n", vendorboot, wd+"vendor_boot.img"))
		log("Cleaning up unpacked vendor boot...")
		check(os.RemoveAll(wd+"vendor_boot"))

		log("Flashing vendor boot...")
		check(cp(wd+"vendor_boot.img", vendorboot))
	}
}

func magiskboot(dir string, args ...string) error {
	return run(mb, dir, args...)
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
	file, err := os.OpenFile(src, os.O_RDONLY, os.ModeAppend)
	if err != nil {
		return err
	}
	data, err := ioutil.ReadAll(file)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0644)
}

func file(path string) string {
	fileType, err := exec.Command(wd+"/bin/file-" + runtime.GOARCH, "--magic-file", wd+"bin/magic.mgc", path).Output()
	if err != nil {
		return fmt.Sprintf("file: %v", err)
	}

	return string(fileType[len(path)+2:len(fileType)-1])
}
