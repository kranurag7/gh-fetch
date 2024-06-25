### gh-fetch

A simple tool to fetch release assets from GitHub repositories interactively.

### Installation 

```bash
gh extension install kranurag7/gh-fetch
```

### Demo 

![asciicast](./demo.gif)

### Usage
```bash
# download assets from a repo
gh fetch -R sigstore/cosign # press enter or d to download
# download from a specific tag
gh fetch -R sigstore/cosign -t v2.0.0 
```
