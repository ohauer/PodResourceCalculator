FROM golang:1.21-alpine

ARG UID=1000
ARG GID=1000
ARG IDUN=foobar

RUN addgroup -g ${GID} ${IDUN} && \
    adduser -D -h /src -G ${IDUN} -u ${UID} ${IDUN}

USER ${IDUN}

COPY ./src /src/
WORKDIR /src

RUN go build -o PodResourceCalculator

