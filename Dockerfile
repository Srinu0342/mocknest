FROM scratch

WORKDIR /app

COPY server .
COPY mocks/ /app/mocks/

EXPOSE 8342

CMD ["./server"]
