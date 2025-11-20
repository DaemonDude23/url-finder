# Dev Docs

## Go init

- Install Go dependencies

```bash
cd ./src/
```

```bash
go mod init daemondude23/url-finder/m
```

```bash
# optionally download deps and update go.sum
cd ./src/
rm -f go.mod ; go mod init daemondude23/url-finder/m ; go mod tidy
```

## Build Executable

```bash
cd ./src/
go build -o ../build/test/url-finder
```

## Trigger goreleaser

```bash
git tag v0.1.0
git push origin v0.1.0
```

## Nix

- Installs some dependencies you can use to run **url-finder**.

```bash
nix-shell
```
