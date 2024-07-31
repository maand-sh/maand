#!/usr/bin/bash
set -ueo pipefail

for plugin in opensearch-ml opensearch-observability opensearch-security-analytics opensearch-notifications opensearch-neural-search; do
  /usr/share/opensearch/bin/opensearch-plugin remove $plugin || true
done

/usr/share/opensearch/bin/opensearch