#!/usr/bin/env python3
"""
Import Ukrainian books from Wikidata into the recommendation system database.

Requirements:
    pip install requests psycopg2-binary python-dotenv
"""

import json
import os
import sys
import time
import urllib.parse
import requests
import psycopg2
import psycopg2.extras
from dotenv import load_dotenv

# ---------------------------------------------------------------------------
# Wikidata SPARQL
# ---------------------------------------------------------------------------

WDQS_URL = "https://query.wikidata.org/sparql"
WDQS_HEADERS = {
    "Accept": "application/sparql-results+json",
    "User-Agent": "UkrainianBooksImporter/1.0 (educational project; contact: import-bot)",
}

# Books, novels, novellas, short story collections, poetry collections, etc.
BOOK_TYPES = " ".join([
    "wd:Q571",        # book
    "wd:Q7725634",    # literary work
    "wd:Q8261",       # novel
    "wd:Q49084",      # short story
    "wd:Q18534530",   # novella
    "wd:Q46261",      # poetry collection
    "wd:Q17517379",   # collection of short stories
    "wd:Q386724",     # work
])

SPARQL_QUERY = """
SELECT DISTINCT ?book ?title ?description ?year ?image
       (GROUP_CONCAT(DISTINCT ?genreLabel; SEPARATOR="|") AS ?genres)
WHERE {
  VALUES ?bookType { %(book_types)s }
  ?book wdt:P31 ?bookType .

  # Language of work must be Ukrainian
  ?book wdt:P407 wd:Q8798 .

  # Strictly exclude Russian origin
  FILTER NOT EXISTS { ?book wdt:P495 wd:Q159 . }   # country != Russia
  FILTER NOT EXISTS { ?book wdt:P364 wd:Q7737 . }  # original language != Russian

  # Must have Ukrainian-language label
  ?book rdfs:label ?title FILTER(LANG(?title) = "uk") .

  OPTIONAL {
    ?book schema:description ?description
    FILTER(LANG(?description) = "uk") .
  }
  OPTIONAL {
    ?book wdt:P577|wdt:P575 ?date .
    BIND(YEAR(?date) AS ?year)
  }
  OPTIONAL {
    ?book wdt:P136 ?genre .
    ?genre rdfs:label ?genreLabel FILTER(LANG(?genreLabel) = "uk") .
  }
  OPTIONAL { ?book wdt:P18 ?image . }
}
GROUP BY ?book ?title ?description ?year ?image
LIMIT 5000
""" % {"book_types": BOOK_TYPES}


def fetch_wikidata_books() -> list[dict]:
    print("Запит до Wikidata SPARQL (може зайняти 30–90 сек)...")
    resp = requests.get(
        WDQS_URL,
        params={"query": SPARQL_QUERY, "format": "json"},
        headers=WDQS_HEADERS,
        timeout=180,
    )
    resp.raise_for_status()
    bindings = resp.json()["results"]["bindings"]
    print(f"  Отримано {len(bindings)} рядків")
    return bindings


def parse_books(bindings: list[dict]) -> list[dict]:
    seen: dict[str, dict] = {}
    for b in bindings:
        qid = b["book"]["value"].rsplit("/", 1)[-1]
        if qid in seen:
            continue

        title = b["title"]["value"].strip()
        if not title:
            continue

        description = b.get("description", {}).get("value", "").strip()
        if not description:
            description = title

        year_raw = b.get("year", {}).get("value")
        year = int(year_raw) if year_raw else None

        image_url = b.get("image", {}).get("value", "") or ""

        genres_raw = b.get("genres", {}).get("value", "") or ""
        genres = [g.strip() for g in genres_raw.split("|") if g.strip()]

        seen[qid] = {
            "wikidata_id": qid,
            "title": title,
            "description": description,
            "release_year": year,
            "image_url": image_url,
            "genres": genres,
        }

    return list(seen.values())


# ---------------------------------------------------------------------------
# Database
# ---------------------------------------------------------------------------

def build_dsn(raw_url: str) -> str:
    """Strip pgx-only params and return a psycopg2-compatible DSN."""
    parsed = urllib.parse.urlparse(raw_url)
    params = urllib.parse.parse_qs(parsed.query)

    # Keep only params that psycopg2 understands
    allowed = {"sslmode", "sslcert", "sslkey", "sslrootcert", "connect_timeout"}
    clean = {k: v[0] for k, v in params.items() if k in allowed}

    new_query = urllib.parse.urlencode(clean)
    clean_url = parsed._replace(query=new_query).geturl()
    return clean_url


def ensure_genre(cur, name: str) -> int:
    cur.execute("SELECT id FROM genres WHERE name = %s", (name,))
    row = cur.fetchone()
    if row:
        return row[0]
    cur.execute(
        "INSERT INTO genres (name) VALUES (%s) ON CONFLICT (name) DO UPDATE SET name = EXCLUDED.name RETURNING id",
        (name,),
    )
    return cur.fetchone()[0]


def book_exists(cur, wikidata_id: str, title: str, year) -> bool:
    cur.execute(
        """
        SELECT EXISTS (
            SELECT 1 FROM items
            WHERE type = 'book'
              AND (
                metadata->>'wikidata_id' = %s
                OR (title = %s AND release_year IS NOT DISTINCT FROM %s)
              )
        )
        """,
        (wikidata_id, title, year),
    )
    return cur.fetchone()[0]


def insert_book(cur, book: dict) -> int | None:
    metadata = json.dumps({
        "source": "wikidata",
        "wikidata_id": book["wikidata_id"],
    })
    cur.execute(
        """
        INSERT INTO items (type, title, description, release_year, image_url, metadata)
        VALUES ('book', %s, %s, %s, %s, %s)
        ON CONFLICT (title, type) DO NOTHING
        RETURNING id
        """,
        (
            book["title"],
            book["description"],
            book["release_year"],
            book["image_url"],
            metadata,
        ),
    )
    row = cur.fetchone()
    return row[0] if row else None


# ---------------------------------------------------------------------------
# Main
# ---------------------------------------------------------------------------

def main():
    # Fix Windows console encoding
    if hasattr(sys.stdout, "reconfigure"):
        sys.stdout.reconfigure(encoding="utf-8", errors="replace")
    if hasattr(sys.stderr, "reconfigure"):
        sys.stderr.reconfigure(encoding="utf-8", errors="replace")

    env_path = os.path.join(os.path.dirname(__file__), "..", ".env")
    load_dotenv(dotenv_path=env_path)

    db_url = os.getenv("DATABASE_URL")
    if not db_url:
        print("❌ DATABASE_URL не знайдено у .env", file=sys.stderr)
        sys.exit(1)

    # 1. Fetch from Wikidata
    bindings = fetch_wikidata_books()
    books = parse_books(bindings)
    print(f"\nУнікальних книг після парсингу: {len(books)}")

    if not books:
        print("❌ Нічого не знайдено.")
        sys.exit(0)

    # 2. Connect to DB
    dsn = build_dsn(db_url)
    conn = psycopg2.connect(dsn)
    conn.autocommit = False
    cur = conn.cursor()

    inserted = 0
    skipped = 0
    errors = 0

    print("\nЗаписую у базу даних...")
    for i, book in enumerate(books):
        try:
            if book_exists(cur, book["wikidata_id"], book["title"], book["release_year"]):
                skipped += 1
                continue

            item_id = insert_book(cur, book)

            if item_id is None:
                skipped += 1
                continue

            for genre_name in book["genres"]:
                genre_id = ensure_genre(cur, genre_name)
                cur.execute(
                    "INSERT INTO item_genres (item_id, genre_id) VALUES (%s, %s) ON CONFLICT DO NOTHING",
                    (item_id, genre_id),
                )

            inserted += 1

        except Exception as exc:
            conn.rollback()
            errors += 1
            print(f"  ⚠️  Пропущено '{book['title']}': {exc}")
            continue

        # Commit every 100 rows
        if inserted % 100 == 0 and inserted > 0:
            conn.commit()
            print(f"  {i + 1}/{len(books)} оброблено, додано {inserted}...")

    conn.commit()
    cur.close()
    conn.close()

    print(f"\n✅ Готово!")
    print(f"  Додано книг:            {inserted}")
    print(f"  Пропущено (дублікати):  {skipped}")
    print(f"  Помилок:                {errors}")


if __name__ == "__main__":
    main()
