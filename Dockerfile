FROM scratch

WORKDIR /app

COPY server .

EXPOSE 8342

CMD ["./server"]
