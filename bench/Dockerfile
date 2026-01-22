ARG IMAGE=alpine
ARG TAG=edge
ARG ARCH=amd64
ARG OS=linux
ARG CLIENT=memcachex

FROM golang:1.25-alpine as build
ARG ARCH
ARG OS
ARG CLIENT

WORKDIR /build
COPY . .
RUN CGO_ENABLED=0 GOOS=${OS} GOARCH=${ARCH} go build -o bench -tags ${CLIENT} .

FROM ${IMAGE}:${TAG}
COPY --from=build /build/bench /bench

CMD ["/bench"]
