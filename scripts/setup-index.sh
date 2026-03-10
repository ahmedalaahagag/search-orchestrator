#!/usr/bin/env bash
set -euo pipefail

OS_URL="${OPENSEARCH_URL:-http://localhost:9200}"
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
CHUNK_LINES=2000  # 1000 docs per chunk (2 lines each: action + source)

MARKETS=("us" "ca" "gb")

echo "Waiting for OpenSearch at ${OS_URL}..."
until curl -sf "${OS_URL}/_cluster/health" > /dev/null 2>&1; do
  sleep 1
done
echo "OpenSearch is ready."

create_index() {
  local index="$1"

  curl -sf -X DELETE "${OS_URL}/${index}" > /dev/null 2>&1 || true

  curl -sf -X PUT "${OS_URL}/${index}" \
    -H 'Content-Type: application/json' \
    -d '{
    "settings": {
      "number_of_shards": 1,
      "number_of_replicas": 0,
      "similarity": {
        "default": {
          "type": "boolean"
        }
      },
      "analysis": {
        "char_filter": {
          "special_symbols_filter": {
            "type": "mapping",
            "mappings": [
              "$ => _dollar_symbol_",
              "( => ",
              ") => "
            ]
          }
        },
        "filter": {
          "shingle_filter": {
            "type": "shingle",
            "min_shingle_size": 2,
            "max_shingle_size": 4,
            "output_unigrams": false,
            "filler_token": ""
          },
          "EnglishPossessiveFilter": {
            "type": "stemmer",
            "name": "possessive_english"
          },
          "custom_sw_filter": {
            "type": "stop",
            "stopwords": ["color","size","with","without","cheap","sale","for","of","and","or","a","the","only","to","in","at","by","on","x","is"]
          }
        },
        "analyzer": {
          "filter_analyzer": {
            "type": "custom",
            "tokenizer": "standard",
            "char_filter": ["special_symbols_filter"],
            "filter": ["asciifolding", "lowercase"]
          },
          "concept_analyzer": {
            "type": "custom",
            "tokenizer": "keyword",
            "char_filter": ["special_symbols_filter"],
            "filter": ["asciifolding", "lowercase"]
          },
          "shingle_analyzer": {
            "tokenizer": "standard",
            "char_filter": ["special_symbols_filter"],
            "filter": ["asciifolding", "lowercase", "custom_sw_filter", "EnglishPossessiveFilter", "porter_stem", "shingle_filter"]
          },
          "text_analyzer": {
            "type": "custom",
            "tokenizer": "standard",
            "char_filter": ["special_symbols_filter"],
            "filter": ["asciifolding", "lowercase", "custom_sw_filter", "EnglishPossessiveFilter", "porter_stem"]
          }
        }
      }
    },
    "mappings": {
      "dynamic_templates": [
        {
          "bool_fields": {
            "match_mapping_type": "boolean",
            "match": "*",
            "mapping": { "type": "boolean" }
          }
        },
        {
          "numeric_fields": {
            "match_mapping_type": "long",
            "match": "*",
            "mapping": { "type": "long" }
          }
        },
        {
          "numeric_fields_double": {
            "match_mapping_type": "double",
            "match": "*",
            "mapping": { "type": "double" }
          }
        },
        {
          "all_fields": {
            "match": "*",
            "mapping": {
              "type": "keyword",
              "fields": {
                "text": {
                  "type": "text",
                  "analyzer": "text_analyzer",
                  "norms": false,
                  "index_options": "docs",
                  "fielddata": true
                },
                "filter": {
                  "type": "text",
                  "analyzer": "filter_analyzer",
                  "norms": false,
                  "index_options": "docs"
                },
                "concept": {
                  "type": "text",
                  "analyzer": "concept_analyzer",
                  "norms": false,
                  "index_options": "docs",
                  "fielddata": true
                },
                "shingle": {
                  "type": "text",
                  "analyzer": "shingle_analyzer",
                  "norms": false,
                  "index_options": "docs",
                  "fielddata": true
                }
              }
            }
          }
        }
      ],
      "properties": {
        "active":           { "type": "boolean" },
        "is_addon":         { "type": "boolean" },
        "is_hidden":        { "type": "boolean" },
        "sold_out":         { "type": "boolean" },
        "hide_on_sold_out": { "type": "boolean" },
        "is_all_region":    { "type": "boolean" },
        "index":            { "type": "long" },
        "updated_at":       { "type": "date" }
      }
    }
  }' > /dev/null
}

seed_index() {
  local index="$1"
  local seed_gz="$2"

  SEED_FILE=$(mktemp)
  gunzip -c "${seed_gz}" > "${SEED_FILE}"

  TOTAL_LINES=$(wc -l < "${SEED_FILE}")
  OFFSET=1
  BATCH=0

  while [ "$OFFSET" -le "$TOTAL_LINES" ]; do
    BATCH=$((BATCH + 1))
    sed -n "${OFFSET},$((OFFSET + CHUNK_LINES - 1))p" "${SEED_FILE}" | \
      curl -sf -X POST "${OS_URL}/${index}/_bulk" \
        -H 'Content-Type: application/x-ndjson' \
        --data-binary @- > /dev/null
    OFFSET=$((OFFSET + CHUNK_LINES))
  done

  rm -f "${SEED_FILE}"

  curl -sf -X POST "${OS_URL}/${index}/_refresh" > /dev/null
  COUNT=$(curl -sf "${OS_URL}/${index}/_count" | python3 -c "import sys,json; print(json.load(sys.stdin)['count'])")
  echo "  ${index}: ${COUNT} documents"
}

for market in "${MARKETS[@]}"; do
  INDEX="hellofresh_${market}_productsonline"
  SEED_GZ="${SCRIPT_DIR}/seed_data_${market}.ndjson.gz"

  if [ ! -f "${SEED_GZ}" ]; then
    echo "Skipping ${INDEX}: seed file not found (${SEED_GZ})"
    continue
  fi

  echo "Setting up ${INDEX}..."
  create_index "${INDEX}"
  seed_index "${INDEX}" "${SEED_GZ}"
done

echo "Done."
