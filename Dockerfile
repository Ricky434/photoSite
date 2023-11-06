FROM golang:alpine
WORKDIR /usr/src/app
COPY go.mod go.sum ./
RUN go mod download && go mod verify

RUN go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest
RUN apk add --no-cache exiftool imagemagick ffmpeg

COPY . .
RUN go build -v -o /usr/local/bin/app_cli ./cmd/cli/
RUN go build -v -o /usr/local/bin/app_server ./cmd/server/

ARG DB_USER="user"
ARG DB_PASSWORD="password"
ARG DB_HOST="db"
ARG DB_NAME="sitoWow"
ARG PORT=4000
ARG ADMIN_NAME=admin
ARG ADMIN_PASSWORD=password

ENV DB_DSN="postgres://$DB_USER:$DB_PASSWORD@$DB_HOST/$DB_NAME?sslmode=disable"
ENV ADMIN_NAME=$ADMIN_NAME
ENV ADMIN_PASSWORD=$ADMIN_PASSWORD
ENV STORAGE_DIR="/data/storage"

CMD migrate -path=./migrations -database=$DB_DSN up \
    && app_cli -db-dsn $DB_DSN createAdmin -name $ADMIN_NAME -password $ADMIN_PASSWORD; \
    app_server -db-dsn $DB_DSN -port $PORT -storage-dir $STORAGE_DIR
