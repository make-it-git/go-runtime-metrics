Example showing different go runtime metrics

```shell
docker compose up -d --force-recreate --build
sleep 3m
curl localhost:8080/leak-goroutines
sleep 3m

docker compose up -d --force-recreate --build app
sleep 3m
curl localhost:8080/gc-pressure
sleep 3m

docker compose up -d --force-recreate --build app
sleep 3m
curl localhost:8080/memory-growth
sleep 3m

docker compose up -d --force-recreate --build app
sleep 3m
curl localhost:8080/alloc-churn
sleep 3m

docker compose up -d --force-recreate --build app
sleep 3m
curl localhost:8080/syscall-pressure
sleep 3m

docker compose up -d --force-recreate --build app
```