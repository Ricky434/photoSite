# ==== Build
FROM golang:alpine AS build
WORKDIR /usr/src/app
COPY go.mod go.sum ./
RUN go mod download && go mod verify

COPY . .
RUN go build -v -o /usr/local/bin/app_cli ./cmd/cli/
RUN go build -v -o /usr/local/bin/app_server ./cmd/server/

# ==== Deploy
FROM alpine
WORKDIR /app
COPY --from=build /usr/local/bin/app_cli /usr/local/bin/app_server /usr/local/bin/

# Need to specify target dir for each folder otherwise contents will be copied
COPY --from=build /usr/src/app/ui/static/ /app/static
COPY --from=build /usr/src/app/migrations/ /app/migrations
ENV STATIC_DIR="/app/static"
ENV MIGRATIONS_DIR="/app/migrations"

RUN apk add --no-cache exiftool imagemagick ffmpeg

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
ENV PORT=$PORT
ENV STORAGE_DIR="/data/storage"

CMD app_cli -db-dsn $DB_DSN createAdmin -name $ADMIN_NAME -password $ADMIN_PASSWORD; \
    app_server -db-dsn $DB_DSN -db-migrations $MIGRATIONS_DIR -static-dir $STATIC_DIR -port $PORT -storage-dir $STORAGE_DIR
