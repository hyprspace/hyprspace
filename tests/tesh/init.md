# tesh: Testable init command example for hyprspace

The init command generates random keys, so we use wildcards (`...`)
to match non-deterministic portions like peer IDs.

## Initialize a new config

```console tesh-session="init"
$ hyprspace init -i test0 -c /tmp/hyprspace-test.json
Initialized new config at /tmp/hyprspace-test.json
Add this entry to your other peers:
{
  "name": "...",
  "id": "12D3KooW..."
}
```
