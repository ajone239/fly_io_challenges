build:
	go build

clean:
	go clean
	rm -rf store

test: build
	maelstrom test -w unique-ids --bin ./maelstrom-uuid --time-limit 30 --rate 1000 --node-count 3 --availability total --nemesis partition
