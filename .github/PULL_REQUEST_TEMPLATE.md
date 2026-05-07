## Summary

*

## Verification

Run the fast local CI before pushing:

```bash
bash ./scripts/ci-local.sh clean
```

Run the full local CI when touching browser automation or Xiaohongshu publishing behavior:

```bash
bash ./scripts/ci-local.sh full
```

Add any other relevant commands:

*

## Safety checklist

* [ ] I did not commit `.env`, `configs/config.yaml`, API keys, tokens, passwords, or other secrets.
* [ ] I did not commit private Markdown documents, generated private notes, local output, logs, or local artifacts.
* [ ] Any config examples are sanitized and safe to publish.
* [ ] Release packaging or workflow changes were checked with `bash ./scripts/secret-scan.sh --current --history`.
