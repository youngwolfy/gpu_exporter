# syntax=docker/dockerfile:1
# Пример образа gpu-exporter: libdcgm и все зависимости живут внутри образа,
# на хосте нужны только драйвер NVIDIA и NVIDIA Container Toolkit
# (https://docs.nvidia.com/datacenter/cloud-native/container-toolkit/) —
# toolkit прокидывает драйверные библиотеки (libnvidia-ml и т.п.) в контейнер.
#
# Сборка и запуск:
#
#   docker build --build-arg VERSION=0.5.0 --build-arg DCGM_CUDA_MAJOR=12 \
#     -t gpu-exporter:0.5.0-cuda12 -f examples/docker/Dockerfile .
#   docker run -d --name gpu-exporter --restart unless-stopped \
#     --gpus all --cap-add SYS_ADMIN \
#     -p 127.0.0.1:9990:9990 \
#     gpu-exporter:0.5.0-cuda12
#
# --cap-add SYS_ADMIN нужен только для профилирующих метрик (gpu_prof_*);
# без него экспортер сам откатится на базовые поля.

FROM golang:1.26.5-trixie AS build

ARG VERSION=dev

# Для сборки (CGO) нужны заголовки DCGM.
RUN apt-get update \
    && apt-get upgrade -y \
    && apt-get install -y --no-install-recommends ca-certificates curl \
    && curl -fsSL https://developer.download.nvidia.com/compute/cuda/repos/debian13/x86_64/cuda-keyring_1.1-1_all.deb -o /tmp/cuda-keyring.deb \
    && dpkg -i /tmp/cuda-keyring.deb && rm /tmp/cuda-keyring.deb \
    && apt-get update \
    && apt-get install -y --no-install-recommends datacenter-gpu-manager-4-dev \
    && apt-get clean \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go test ./... \
    && go vet ./... \
    && go build -trimpath -ldflags="-s -w -X main.version=${VERSION}" -o /gpu-exporter .

FROM ubuntu:24.04

ARG DCGM_CUDA_MAJOR=12
ARG VERSION=dev

LABEL org.opencontainers.image.title="gpu-exporter" \
      org.opencontainers.image.description="Prometheus exporter for NVIDIA GPU telemetry via DCGM" \
      org.opencontainers.image.version="${VERSION}" \
      org.opencontainers.image.base.name="docker.io/library/ubuntu:24.04" \
      org.opencontainers.image.licenses="MIT"

# В runtime-слое нужна libdcgm.so.4 с модулями. NVIDIA рекомендует
# устанавливать CUDA-specific DCGM-пакет вместе с recommended-пакетами:
# они добавляют закрытые DCGM-модули, включая часть profiling-функциональности.
RUN apt-get update \
    && apt-get upgrade -y \
    && apt-get install -y --no-install-recommends ca-certificates curl \
    && curl -fsSL https://developer.download.nvidia.com/compute/cuda/repos/ubuntu2404/x86_64/cuda-keyring_1.1-1_all.deb -o /tmp/cuda-keyring.deb \
    && dpkg -i /tmp/cuda-keyring.deb && rm /tmp/cuda-keyring.deb \
    && apt-get update \
    && apt-get install -y --install-recommends "datacenter-gpu-manager-4-cuda${DCGM_CUDA_MAJOR}" \
    && apt-get purge -y curl && apt-get autoremove -y \
    && apt-get clean \
    && rm -rf /var/lib/apt/lists/*

COPY --from=build /gpu-exporter /usr/local/bin/gpu-exporter
COPY LICENSE /usr/share/licenses/gpu-exporter/LICENSE

# Метки для NVIDIA Container Toolkit: какие GPU и какие драйверные
# библиотеки прокидывать в контейнер.
ENV NVIDIA_VISIBLE_DEVICES=all
ENV NVIDIA_DRIVER_CAPABILITIES=utility,compute

ENV GPU_EXPORTER_ADDR=0.0.0.0:9990
EXPOSE 9990
STOPSIGNAL SIGTERM

ENTRYPOINT ["/usr/local/bin/gpu-exporter"]
