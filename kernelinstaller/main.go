package main

import (
	"archive/zip"
	"fmt"
	"io"
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

	BUFFERSIZE int64 = 4096
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
	} else if strings.Contains(typeKernel, "Zip archive") {
		typeKernel = "zip"
	} else {
		check(fmt.Errorf("[%s] is not an Android kernel: %s", kernel, typeKernel))
	}
	log("Successfully validated kernel image as " + file(kernel))

	if typeKernel == "linux" {
		log("Nothing to do for kernel")
	} else if typeKernel == "boot" || typeKernel == "zip" {
		check(os.MkdirAll(wd+"kernel/tmp", 0644))

		switch typeKernel {
		case "boot":
			log("Unpacking kernel image to [" + wd+"kernel]...")
			check(magiskboot(wd+"kernel", "unpack", "-h", kernel))

			log("Unpacking kernel image ramdisk to [" + wd+"kernel/tmp]...")
			check(magiskboot(wd+"kernel/tmp", "cpio", wd+"kernel/ramdisk.cpio", "extract"))
		case "zip":
			log("Unpacking kernel zip to [" + wd+"kernel/tmp]...")
			archive, err := zip.OpenReader(kernel)
			check(err)
			defer archive.Close()

			for _, f := range archive.File {
				filePath := filepath.Join(wd+"kernel/tmp", f.Name)

				if f.FileInfo().IsDir() {
					os.MkdirAll(filePath, os.ModePerm)
					continue
				}

				check(os.MkdirAll(filepath.Dir(filePath), os.ModePerm))
				dstFile, err := os.OpenFile(filePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
				check(err)

				fileInArchive, err := f.Open()
				check(err)

				_, err = io.Copy(dstFile, fileInArchive)
				check(err)

				dstFile.Close()
				fileInArchive.Close()
			}
		}

		log("Walking for kernel and dtb...")
		kernel = ""
		dtb = ""
		err := filepath.Walk(wd+"kernel/tmp", func(path string, info os.FileInfo, err error) error {
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
				if strings.Contains(path, "dtb") {
					log("Found kernel+dtb: " + path)
					return io.EOF
				}
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
		check(os.Rename(kernel, wd+"kernel.tmp"))
		kernel = wd+"kernel.tmp"
		if dtb != "" {
			check(os.Rename(dtb, wd+"dtb.tmp"))
			dtb = wd+"dtb.tmp"
		}
		check(os.RemoveAll(wd+"kernel"))
	} else {
		check(fmt.Errorf("What are we supposed to do, exactly?"))
	}
	
	if dtb == "" {
		log("Trying to split kernel for dtb...")
		check(os.MkdirAll(wd+"dtb", 0644))
		if err := magiskboot(wd+"dtb", "split", kernel); err == nil {
			kernel = wd+"dtb/kernel"
			dtb = wd+"dtb/kernel_dtb"

			typeKernel = file(kernel)
			if !strings.Contains(typeKernel, "Linux kernel") {
				check(fmt.Errorf("[%s] is not an Android kernel: %s", kernel, typeKernel))
			}
		}
	}
	if dtb != "" {
		typeDTB := file(dtb)
		if !strings.Contains(typeDTB, "Device Tree Blob") {
			check(fmt.Errorf("[%s] is not a Device Tree Blob: %s", dtb, typeDTB))
		}
		log("Successfully validated dtb as " + typeDTB)
	} else {
		log("No device tree blob found, ignoring...")
	}

	log("Unpacking boot to [" + wd+"boot]...")
	check(os.MkdirAll(wd+"boot", 0644))
	check(magiskboot(wd+"boot", "unpack", "-n", boot))

	log("Injecting kernel into boot...")
	check(os.Rename(kernel, wd+"boot/kernel"))
	if dtb != "" {
		log("Injecting dtb into boot...")
		check(os.Rename(dtb, wd+"boot/dtb"))
	} else if file(dtb) != "" {
		log("Removing dtb from boot...")
		check(os.RemoveAll(wd+"boot/dtb"))
	}

	log("Repacking boot...")
	check(magiskboot(wd+"boot", "repack", "-n", boot, wd+"new.b.img"))
	log("Cleaning up unpacked boot...")
	if dtb != "" {
		check(os.Rename(wd+"boot/dtb", dtb))
	}
	check(os.RemoveAll(wd+"boot"))

	typeBoot = file(wd+"new.b.img")
	if !strings.Contains(typeBoot, "Android bootimg") {
		check(fmt.Errorf("Failed to repack boot"))
	}
	log("Successfully repacked boot as " + typeBoot)

	log("Flashing boot...")
	check(cp(wd+"new.b.img", boot))

	if vendorboot != "" && dtb != "" {
		log("Unpacking vendor boot to [" + wd+"vendor_boot]...")
		check(os.MkdirAll(wd+"vendor_boot", 0644))
		check(magiskboot(wd+"vendor_boot", "unpack", "-n", vendorboot))

		log("Injecting dtb into vendor boot...")
		check(os.Rename(dtb, wd+"vendor_boot/dtb"))

		log("Repacking vendor boot...")
		check(magiskboot(wd+"vendor_boot", "repack", "-n", vendorboot, wd+"new.vb.img"))
		log("Cleaning up unpacked vendor boot...")
		check(os.RemoveAll(wd+"vendor_boot"))

		typeVendorBoot := file(wd+"new.vb.img")
		if !strings.Contains(typeVendorBoot, "data") {
			check(fmt.Errorf("Failed to repack vendor boot"))
		}
		log("Successfully repacked vendor boot as " + typeVendorBoot)

		log("Flashing vendor boot...")
		check(cp(wd+"new.vb.img", vendorboot))
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
	source, err := os.Open(src)
	if err != nil {
		return err
	}
	defer source.Close()

	destination, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destination.Close()

	if err != nil {
		panic(err)
	}

	buf := make([]byte, BUFFERSIZE)
	for {
		n, err := source.Read(buf)
		if err != nil && err != io.EOF {
			return err
		}
		if n == 0 {
			break
		}

		if _, err := destination.Write(buf[:n]); err != nil {
			return err
		}
	}
	return err
}

func file(path string) string {
	fileType, err := exec.Command(wd+"/bin/file-" + runtime.GOARCH, "--magic-file", wd+"bin/magic.mgc", path).Output()
	if err != nil {
		return ""
	}

	return string(fileType[len(path)+2:len(fileType)-1])
}
