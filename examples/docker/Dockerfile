# syntax=docker/dockerfile:1
# Пример образа gpu-exporter: libdcgm и все зависимости живут внутри образа,
# на хосте нужны только драйвер NVIDIA и NVIDIA Container Toolkit
# (https://docs.nvidia.com/datacenter/cloud-native/container-toolkit/) —
# toolkit прокидывает драйверные библиотеки (libnvidia-ml и т.п.) в контейнер.
#
# Сборка и запуск:
#
#   docker build --build-arg DCGM_CUDA_MAJOR=12 \
#     -t gpu-exporter:0.3.0-cuda12 -f examples/docker/Dockerfile .
#   docker run -d --name gpu-exporter --restart unless-stopped \
#     --gpus all --cap-add SYS_ADMIN \
#     -p 127.0.0.1:9990:9990 \
#     gpu-exporter:0.3.0-cuda12
#
# --cap-add SYS_ADMIN нужен только для профилирующих метрик (gpu_prof_*);
# без него экспортер сам откатится на базовые поля.

FROM golang:1.26-bookworm AS build

# Для сборки (CGO) нужны заголовки DCGM.
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates curl \
    && curl -fsSL https://developer.download.nvidia.com/compute/cuda/repos/debian12/x86_64/cuda-keyring_1.1-1_all.deb -o /tmp/cuda-keyring.deb \
    && dpkg -i /tmp/cuda-keyring.deb && rm /tmp/cuda-keyring.deb \
    && apt-get update \
    && apt-get install -y --no-install-recommends datacenter-gpu-manager-4-dev \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -trimpath -ldflags="-s -w" -o /gpu-exporter .

FROM debian:bookworm-slim

ARG DCGM_CUDA_MAJOR=12

# В runtime-слое нужна libdcgm.so.4 с модулями. NVIDIA рекомендует
# устанавливать CUDA-specific DCGM-пакет вместе с recommended-пакетами:
# они добавляют закрытые DCGM-модули, включая часть profiling-функциональности.
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates curl \
    && curl -fsSL https://developer.download.nvidia.com/compute/cuda/repos/debian12/x86_64/cuda-keyring_1.1-1_all.deb -o /tmp/cuda-keyring.deb \
    && dpkg -i /tmp/cuda-keyring.deb && rm /tmp/cuda-keyring.deb \
    && apt-get update \
    && apt-get install -y --install-recommends "datacenter-gpu-manager-4-cuda${DCGM_CUDA_MAJOR}" \
    && apt-get purge -y curl && apt-get autoremove -y \
    && rm -rf /var/lib/apt/lists/*

COPY --from=build /gpu-exporter /usr/local/bin/gpu-exporter

# Метки для NVIDIA Container Toolkit: какие GPU и какие драйверные
# библиотеки прокидывать в контейнер.
ENV NVIDIA_VISIBLE_DEVICES=all
ENV NVIDIA_DRIVER_CAPABILITIES=utility,compute

ENV GPU_EXPORTER_ADDR=0.0.0.0:9990
EXPOSE 9990

ENTRYPOINT ["/usr/local/bin/gpu-exporter"]
