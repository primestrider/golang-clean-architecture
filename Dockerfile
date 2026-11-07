# syntax=docker/dockerfile:1

# ---- Tahap build: kompilasi semua binary ----
FROM golang:1.27.3 AS build
WORKDIR /src

# Unduh dependency lebih dulu supaya layer ini ter-cache selama go.mod dan
# go.sum tidak berubah.
COPY go.mod go.sum ./
RUN go mod download

COPY . .
# CGO_ENABLED=0 menghasilkan binary statis yang bisa berjalan di image
# distroless tanpa libc. -trimpath dan -ldflags="-s -w" memperkecil binary
# dan menghapus path lokal dari hasil build.
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/web ./cmd/web \
 && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/worker ./cmd/worker \
 && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/cli ./cmd/cli

# ---- Tahap runtime: image minimal tanpa shell ----
# distroless/static berisi sertifikat CA dan data zona waktu, tetapi tidak
# punya shell maupun package manager, sehingga permukaan serangannya kecil.
FROM gcr.io/distroless/static:nonroot
WORKDIR /app
COPY --from=build /out/ /app/
COPY config.json /app/config.json

# Jalankan sebagai user non-root.
USER nonroot:nonroot
EXPOSE 3000 9090 9091 50051

# Default menjalankan API. Worker dijalankan dengan entrypoint /app/worker.
ENTRYPOINT ["/app/web"]
