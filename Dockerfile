# parent image
FROM golang:1.15.7-alpine3.12 AS builder

# workspace directory
WORKDIR /app

# copy `go.mod` and `go.sum`
ADD go.mod go.sum ./

# install dependencies
RUN go mod download

# copy source code
COPY . .

# build executable
RUN go build -o ./bin/smartdial .

##################################

# parent image
FROM alpine:3.12.2

#install nano
RUN apk update && apk add --no-cache curl nano wget bash

# workspace directory
WORKDIR /app

# copy binary file from the `builder` stage
COPY --from=builder /app/bin/smartdial ./

# Export necessary port
EXPOSE 7070

# set entrypoint
CMD [ "/app/smartdial", "s"]