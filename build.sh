#!/bin/sh
set -e

go mod tidy
go mod download

mkdir -p build

echo "Linux"
go build -o "build/op-linux"

echo "Android"
echo "NDK路径: ${ANDROID_NDK_HOME}"
if [ -z "$(which gomobile)" ]; then
  echo "没有安装gomobile"
  go install golang.org/x/mobile/cmd/gomobile@latest
  gomobile init
else
  echo "已经安装gomobile"
fi
go get -d golang.org/x/mobile/cmd/gomobile
gomobile bind -target=android/arm64,android/arm,android/386,android/amd64 -o "build/android.aar"  ./qc ./dns ./op
rm "build/android-sources.jar"

go mod tidy
