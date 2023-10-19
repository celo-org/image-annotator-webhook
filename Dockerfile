FROM golang:1.21 AS build

ENV GOOS=linux
ENV GOARCH=amd64
ENV CGO_ENABLED=0

WORKDIR /webhook
COPY . /webhook

RUN --mount=type=cache,target=/root/.cache/go-build,sharing=private \
  go build -o bin/image-annotator-webhook .

# ---
# FROM scratch AS run
FROM ubuntu:latest

COPY --from=build /webhook/bin/image-annotator-webhook /usr/local/bin/

CMD ["image-annotator-webhook"]
