FROM golang:buster as build
WORKDIR /app
ADD ./ /app
RUN ./build.sh

FROM scratch
COPY --from=build /app/prosody-filer /prosody-filer
ENTRYPOINT ["/prosody-filer"]
