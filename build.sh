#!/bin/bash
set -e #Exit on error

# Build config
export GO111MODULE=off #We don't use go.mod yet
#export JDZIP="JD's Toolbox ($(date +%s)).zip"
export JDZIP="jdtoolbox.zip"
export INSTALLDIR=/sdcard/

# Android target
export GOOS=linux
export GOARCH=arm64

# Build memory
export WD=$PWD
export JDMEN=$WD/menu
export JDKERNINST=$WD/kernelinstaller
export JDTWRPINST=$WD/twrpinstaller

export JDMOD=$WD/module
export JDMENBIN=$JDMOD/bin/jdtoolbox
export JDKERNBIN=$JDMOD/bin/kernelinstaller
export JDTWRPBIN=$JDMOD/bin/twrpinstaller

# Go build the menu
cd "$JDMEN"
go build -o "$JDMENBIN" -ldflags="-s -w"

# Go build the kernel installer
cd "$JDKERNINST"
go build -o "$JDKERNBIN" -ldflags="-s -w"

# Go build the TWRP installer
cd "$JDTWRPINST"
go build -o "$JDTWRPBIN" -ldflags="-s -w"

# Zip the Magisk module ZIP
cd "$JDMOD"
zip -r -0 -v module.zip *
mv module.zip "$WD/$JDZIP"

# Push the ZIP to an ADB-connected device
cd "$WD"
adb push "$WD/$JDZIP" "$INSTALLDIR"
