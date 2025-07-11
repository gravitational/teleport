خب من میخواستم تلپورت رو اون بخشی که نیاز داشتم رو تغیر بدم و بیلد کنم و روی کلاسترم بالا بیارم ولی خب چندباری تلاش ناموفق داشتم
اینسری چیشد؟


خب ابتدا باید از روی گیتهاب کلونش میکردم
git clone https://github.com/gravitational/teleport.git
git checkout v17.5.4

بعد از اینکار که انجام شد میبایست تغیرات لازم رو میدادم روی اپیلیکیشنم تا بتونم اون قابلیت های مورد نیازم رو فعال کنم توش


#TODO: چگونه پیدا کردم  تغیرات دادم؟


خب بعد از اینکه این مرحله بخیر گذشت 
باید پیش نیاز های اولیه رو برای بیلد نصب میکردم و خب من شروع کردم با  استفاده از MAKEFILE دونه دونه جلو میرفتم ارور میخوردم و نصب میکردم
مثلا 
pnpm, nodejs 22 , cargo....
و خب ورژن هاشم نمیدونستم و خب شاید اذیت کننده میشد 
برای همین یکم داکیومنت هاشو خوندم و اینجا دیدم که پیش نیاز هاش نوشته شده که باید نصب شه
https://github.com/gravitational/teleport/blob/master/build.assets/versions.mk


ولی من میخواستم ایمیج هایی که روی چارتمه رو بوجود بیارم ولی اینکار راحت نبود برای همین یه ایده داشتم
برم ببینم ایمیج های اصلی چی هستند؟
بعد ببینم با چه کامندی ران میشن و بعدش  بگردم ببینم کدوم داکر فایل های بین دریایی از داکر فایل توی ریپو برام قراره بیلد شن و کمک کننده به من هستند
( این کارا روی یک وی ام ubuntu 24 انجام شده)

خب برای اینکار به هلم چارت تلپورت مراجعه کردم
https://github.com/gravitational/teleport/blob/master/examples/chart/teleport-cluster/values.yaml

بعد دیدم برای image: چه مقادیری استفاده شده.
ایمیج هایی که پیدا کردم 
public.ecr.aws/gravitational/teleport-operator
public.ecr.aws/gravitational/teleport-distroless
این دوتا بودن البته یکی دوتا ایمیج دیگم بودن که برای enterprise اش بودن و من نمیخاستمشون
بعدایمیج هارو پول کردم.

```bash
➜  build git:(api/v17.5.4) ✗ docker image inspect public.ecr.aws/gravitational/teleport-distroless:15 | grep Config -A 10

        "Config": {
            "Cmd": null,
            "Entrypoint": [
                "/usr/bin/dumb-init",
                "/usr/local/bin/teleport",
                "start",
                "-c",
                "/etc/teleport/teleport.yaml"
            ],
```

```bash
➜  build git:(api/v17.5.4) ✗ docker image inspect public.ecr.aws/gravitational/teleport-operator:15 | grep Config -A 10

        "Config": {
            "Cmd": null,
            "Entrypoint": [
                "/teleport-operator"
            ],
```

خب بدش رفتم مثلا یکی از کامند هارو توی کل پروژه سرچ کردم ببینم کدوم داکر فایل هامه که میتونم بیلدشون نم
/usr/local/bin/teleport
ایول این روش جواب داد
به چی رسییدیم؟
به دو داکر فایل زیر
teleport/build.assets/charts/Dockerfile-distroless
teleport/integrations/operator/Dockerfile


خیلی عالی شد الان برای  ایمیج public.ecr.aws/gravitational/teleport-distroless باید فایل زیر رو بیلد کنم
teleport/build.assets/charts/Dockerfile-distroless


و برای public.ecr.aws/gravitational/teleport-operator هم فایل زیر رو باید بیلد کنم
teleport/integrations/operator/Dockerfile


یچیزی که خیلی کمکم کرد این بود که pnpm ام برای گرفتن پکیج هاش 403 میخورد و من بخاطر همین از یه میرر چینی استفاده کردم
pnpm set registry https://registry.npmmirror.com

اینم تو وارنگینگ هاش بود که اگه نصب نکنی و بیلد کنی نمیتونی از 2fa استفاده کنی و یچیزایی مثل google auth و استفاده از yubi key رو نشه استفاده کرد
sudo apt install libfido2-dev


یه مرحله ای که باید یادمون نره اینه که من برای تست اولیه make full میزدم و خب میرفتم تو دیوار
و میگفت اوپتیمایزیشن cargo برای بیلد است های وب میره تو دیوار 
خب من چون دانش کافی نداشتم گفتم یکم با تف این قضیه رو حل کنم برای همین اوپتیمیزیشن رو غیرفعال کردم :|
ابتدا فایل های لازم رو پیدا کردم

➜  teleport git:(api/v17.5.4) ✗ find . -name Cargo.toml
./web/packages/shared/libs/ironrdp/Cargo.toml
./lib/srv/desktop/rdp/rdpclient/Cargo.toml
./Cargo.toml
./tool/fdpass-teleport/Cargo.toml
./node_modules/.pnpm/@swc+plugin-styled-components@3.0.2/node_modules/@swc/plugin-styled-components/Cargo.toml

از بین این ها سه فایل زیر مهم بودن چون packge توشون تعریف شده بود
./web/packages/shared/libs/ironrdp/Cargo.toml
./lib/srv/desktop/rdp/rdpclient/Cargo.toml
./tool/fdpass-teleport/Cargo.toml

```yaml
[package.metadata.wasm-pack.profile.release]
wasm-opt = false
```
این رو بهش اضافه کردم و بوم مشکلم حل شد.


خب بعدش باید میرفتیم که ایمیج اول ینی  distroless رو بیلد کنیم
بعد از اینکه هزارتا راه رفته و نرفته رو تست کردم فهمیدم باید مراحل زیر رو طی کرد.

چرا به این نتیجه رسیدم؟
چون make deb من نمیرفت باینری فایل هارو از لوکال من بیاره و دانلودش میکرد برای همنی و خب منم نتونستم اسکریپتشو تغیر بدم 
برای همین مسیر سخت زیر رو طی کردم


```bash
make release
```

```
cp ./build.assets/build-package.sh ./build.assets/build-common.sh  build.assets/charts/Dockerfile-distroless ./build.assets/charts/fetch-debs build
cd build
```

```bash
➜  build git:(api/v17.5.4) ✗ ls -lash artifacts 
total 393M
4.0K drwxrwxr-x 2 ubuntu ubuntu 4.0K Jul  4 08:42 .
4.0K drwxrwxr-x 3 ubuntu ubuntu 4.0K Jul  4 08:46 ..
197M -rw-rw-r-- 1 ubuntu ubuntu 197M Jul  4 08:42 teleport-v17.5.4-linux-amd64-bin.tar.gz
197M -rw-rw-r-- 1 ubuntu ubuntu 197M Jul  4 08:42 teleport-v17.5.4-linux-amd64-centos7-bin.tar.gz
```

```
./build-package.sh -t oss -v 17.5.4 -p deb -a amd64 -s $(pwd)/artifacts/
```

خب این برای ما فایل های .deb میسازه که بنظر تا اینجای کار خوب پیشرفته
```bash
➜  build git:(api/v17.5.4) ✗ ls                
artifacts  build-common.sh  build-package.sh  fdpass-teleport  fetch-debs  tbot  tctl  teleport  teleport_17.5.4_amd64.deb  teleport_17.5.4_amd64.deb.sha256  teleport-update  tsh
```

```Dockerfile-distroless
FROM ubuntu:24.04

ENV DEBIAN_FRONTEND=noninteractive

# نصب پکیج‌های مورد نیاز برای نصب پکیج deb و اجرای Teleport
RUN apt-get update && apt-get install -y \
    ca-certificates \
    dumb-init \
    libpam0g \
    libaudit1 \
    libcap-ng0 \
    libfido2-1 \
    && apt-get clean && rm -rf /var/lib/apt/lists/*

# ورژن و فایل .deb را به صورت آرگومان دریافت می‌کنیم
ARG TELEPORT_VERSION
ARG TELEPORT_RELEASE_INFIX=
ARG TARGETARCH=amd64
ENV TELEPORT_DEB_FILE_NAME=teleport${TELEPORT_RELEASE_INFIX}_${TELEPORT_VERSION}_${TARGETARCH}.deb

# کپی فایل deb
COPY ${TELEPORT_DEB_FILE_NAME} /tmp/

# نصب Teleport
RUN dpkg -i /tmp/${TELEPORT_DEB_FILE_NAME} && rm -f /tmp/${TELEPORT_DEB_FILE_NAME}

# پوشه‌های مورد نیاز
RUN mkdir -p /etc/teleport /var/lib/teleport

# سیگنال برای shutdown مرتب
STOPSIGNAL SIGQUIT

# اجرای Teleport با dumb-init
ENTRYPOINT ["/usr/bin/dumb-init", "/usr/local/bin/teleport", "start", "-c", "/etc/teleport/teleport.yaml"]
```

```
docker buildx build -f Dockerfile-distroless -t irania9o/teleport-distroless:17.5.4 --build-arg TELEPORT_VERSION=17.5.4 .
```
خب کارمون با اولی تموم شد لازمه بریم دومی رو بیلد کنیم تا بتونیم تازه بعدش بریم روی کلاسترمون اجراش کنیم.

```
docker build -f integrations/operator/Dockerfile -t test:latest .
```
یا
```
cd integrations/operator/
make docker-build
```
داکر فایلمو یکم تغییر دارم
```Dockerfile
ARG BASE_IMAGE=gcr.io/distroless/cc-debian12

# BUILDPLATFORM is provided by Docker/buildx
FROM --platform=$BUILDPLATFORM docker.arvancloud.ir/debian:12 as builder
ARG BUILDARCH

## Install dependencies.
RUN apt-get update && apt-get install -y --no-install-recommends \
    curl \
    git \
    ca-certificates \
    unzip \
    # x86_64 dependencies and build tools
    build-essential \
    # ARM dependencies
    libc6-dev-armhf-cross \
    gcc-arm-linux-gnueabihf \
    # ARM64 dependencies
    libc6-dev-arm64-cross \
    gcc-aarch64-linux-gnu \
    # i386 dependencies
    libc6-dev-i386-cross \
    gcc-i686-linux-gnu

# Install Go.
#ARG GOLANG_VERSION
#RUN mkdir -p /opt && cd /opt && curl -fsSL https://storage.googleapis.com/golang/$GOLANG_VERSION.linux-${BUILDARCH}.tar.gz | tar xz && \
#    chmod a+w /var/lib && \
#    chmod a-w /
#ENV GOPATH="/go" \
#    GOROOT="/opt/go" \
#    PATH="$PATH:/opt/go/bin:/go/bin"
ARG GOLANG_VERSION
COPY ../../../go1.23.10.linux-amd64.tar.gz /opt/

RUN mkdir -p /opt && cd /opt && \
    tar -xzf go1.23.10.linux-amd64.tar.gz && \
    rm go1.23.10.linux-amd64.tar.gz && \
    chmod a+w /var/lib && \
    chmod a-w /

ENV GOPATH="/go" \
    GOROOT="/opt/go" \
    PATH="$PATH:/opt/go/bin:/go/bin"


# Install protoc.
ARG PROTOC_VERSION # eg, "3.20.2"
RUN VERSION="$PROTOC_VERSION" && \
  PB_REL='https://github.com/protocolbuffers/protobuf/releases' && \
  PB_FILE="$(mktemp protoc-XXXXXX.zip)" && \
  curl -fsSL -o "$PB_FILE" "$PB_REL/download/v$VERSION/protoc-$VERSION-linux-$(if [ "$BUILDARCH" = "amd64" ]; then echo "x86_64"; else echo "aarch_64"; fi).zip"  && \
  unzip "$PB_FILE" -d /usr/local && \
  rm -f "$PB_FILE"

## Build the operator

WORKDIR /go/src/github.com/gravitational/teleport

# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum
COPY vendor/ ./vendor/

# We have to copy the API before `go mod download` because go.mod has a replace directive for it
COPY api/ api/

# Download and Cache dependencies before building and copying source
# This will prevent re-downloading the operator's dependencies if they have not changed as this
# `run` layer will be cached
#ENV GOPROXY=https://goproxy.cn,direct
#ENV GOPROXY=https://goproxy.io,direct
#RUN go mod download -x

COPY *.go ./
COPY lib/ lib/
COPY gen/ gen/
COPY entitlements/ entitlements/
COPY integrations/lib/embeddedtbot/ integrations/lib/embeddedtbot/
COPY integrations/operator/apis/ integrations/operator/apis/
COPY integrations/operator/controllers/ integrations/operator/controllers/
COPY integrations/operator/main.go integrations/operator/main.go
COPY integrations/operator/namespace.go integrations/operator/namespace.go
COPY integrations/operator/config.go integrations/operator/config.go

# Compiler package should use host-triplet-agnostic name (i.e. "x86-64-linux-gnu-gcc" instead of "gcc")
#  in most cases, to avoid issues on systems with multiple versions of gcc (i.e. buildboxes)
# TARGETOS and TARGETARCH are provided by Docker/buildx, but must be explicitly listed here
ARG COMPILER_NAME
ARG TARGETOS
ARG TARGETARCH

RUN go mod verify

# Build the program
# CGO is required for github.com/gravitational/teleport/lib/system
RUN echo "Targeting $TARGETOS/$TARGETARCH with CC=$COMPILER_NAME" && \
    CGO_ENABLED=1 CC=$COMPILER_NAME GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -mod=vendor -tags "kustomize_disable_go_plugin_support" -a -o /go/bin/teleport-operator github.com/gravitational/teleport/integrations/operator

# Create the image with the build operator on the $TARGETPLATFORM
# TARGETPLATFORM is provided by Docker/buildx
FROM --platform=$TARGETPLATFORM $BASE_IMAGE
WORKDIR /
COPY --from=builder /go/bin/teleport-operator .

ENTRYPOINT ["/teleport-operator"]
```