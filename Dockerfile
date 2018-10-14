FROM golang:1.11-alpine As builder

RUN apk --no-cache --upgrade add ca-certificates \
    && update-ca-certificates --fresh \
    && apk --no-cache add --upgrade \
    git

COPY . /src/users-rw-sql

WORKDIR /src/users-rw-sql

RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o /go/bin/users-rw-sql

FROM scratch

# Copy our static executable from the builder
COPY --from=builder /go/bin/users-rw-sql /bin/users-rw-sql

EXPOSE 8080

ENTRYPOINT ["/bin/users-rw-sql"]