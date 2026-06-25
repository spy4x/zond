# Zond — Internal health probe bridge
#
FROM denoland/deno:alpine-2.2.8

WORKDIR /app

COPY src/ src/
COPY deno.jsonc .

RUN deno cache src/main.ts

ENV ZOND_PORT=8080

EXPOSE 8080

CMD ["deno", "run", "-A", "src/main.ts"]
