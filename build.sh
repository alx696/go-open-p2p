#!/bin/sh
set -e

go mod tidy
go mod download

mkdir -p build

echo "Linux"
go build -o "build/op-linux"

echo "Windows"
sudo apt-get install -y gcc-multilib gcc-mingw-w64
GOOS=windows GOARCH=amd64 CGO_ENABLED=1 CC=x86_64-w64-mingw32-gcc CXX=x86_64-w64-mingw32-g++ go build -o "build/op-windows"

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
