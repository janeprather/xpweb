FROM golang:1.24-bookworm

RUN go install golang.org/x/pkgsite/cmd/pkgsite@latest

RUN useradd -m -s /bin/bash vscode
USER vscode

ENV GOPATH=""
