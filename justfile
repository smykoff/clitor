# Собрать бинарник clitor
build:
	go build -trimpath -o clitor .

# just без аргументов = just build
default: build
