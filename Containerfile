ARG APP_EXECUTABLE=cruiser

FROM docker.io/library/golang:1.20-bullseye as builder

ARG APP_EXECUTABLE

WORKDIR /app

RUN GO111MODULE=on go install github.com/bufbuild/buf/cmd/buf@v1.19.0
RUN go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.30.0
RUN go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.3.0

COPY go.mod ./
COPY go.sum ./

RUN go mod download

COPY . ./

RUN buf generate

RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o ${APP_EXECUTABLE} ./

FROM docker.io/library/debian:bullseye as runtime

ARG APP_EXECUTABLE

WORKDIR /var

RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates curl

COPY --from=builder /app/${APP_EXECUTABLE} /usr/bin/

CMD [ "${APP_EXECUTABLE}", "-h" ]