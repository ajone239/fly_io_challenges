build: *.go
	go build

clean:
	go clean
	rm -rf store

test_echo:
	maelstrom test -w echo --bin ./maelstrom-broadcast --node-count 1 --time-limit 10

test1: build
	maelstrom test -w broadcast --bin maelstrom-broadcast --node-count 1 --time-limit 20 --rate 10

test2: build
	maelstrom test -w broadcast --bin maelstrom-broadcast --node-count 5 --time-limit 20 --rate 10

test3: build
	maelstrom test -w broadcast --bin maelstrom-broadcast \
		--node-count 5 \
		--time-limit 20 \
		--rate 10 \
		--nemesis partition
