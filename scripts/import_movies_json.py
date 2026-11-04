#!/usr/bin/env python3
"""
Import Ukrainian movies from a Wikidata SPARQL JSON export (query.json)
into the recommendation system database.

Usage:
    python scripts/import_movies_json.py [path/to/query.json]
"""

import json
import os
import sys
import urllib.parse
import psycopg2
from dotenv import load_dotenv


def parse_movies(bindings: list[dict]) -> list[dict]:
    seen: dict[str, dict] = {}
    for b in bindings:
        qid = b["film"]["value"].rsplit("/", 1)[-1]
        if qid in seen:
            continue

        title = b.get("title", {}).get("value", "").strip()
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


def build_dsn(raw_url: str) -> str:
    parsed = urllib.parse.urlparse(raw_url)
    params = urllib.parse.parse_qs(parsed.query)
    allowed = {"sslmode", "sslcert", "sslkey", "sslrootcert", "connect_timeout"}
    clean = {k: v[0] for k, v in params.items() if k in allowed}
    new_query = urllib.parse.urlencode(clean)
    return parsed._replace(query=new_query).geturl()


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


def movie_exists(cur, wikidata_id: str, title: str, year) -> bool:
    cur.execute(
        """
        SELECT EXISTS (
            SELECT 1 FROM items
            WHERE type = 'movie'
              AND (
                metadata->>'wikidata_id' = %s
                OR (title = %s AND release_year IS NOT DISTINCT FROM %s)
              )
        )
        """,
        (wikidata_id, title, year),
    )
    return cur.fetchone()[0]


def insert_movie(cur, movie: dict) -> int | None:
    metadata = json.dumps({
        "source": "wikidata",
        "wikidata_id": movie["wikidata_id"],
    })
    cur.execute(
        """
        INSERT INTO items (type, title, description, release_year, image_url, metadata)
        VALUES ('movie', %s, %s, %s, %s, %s)
        ON CONFLICT (title, type) DO NOTHING
        RETURNING id
        """,
        (
            movie["title"],
            movie["description"],
            movie["release_year"],
            movie["image_url"],
            metadata,
        ),
    )
    row = cur.fetchone()
    return row[0] if row else None


def main():
    if hasattr(sys.stdout, "reconfigure"):
        sys.stdout.reconfigure(encoding="utf-8", errors="replace")
    if hasattr(sys.stderr, "reconfigure"):
        sys.stderr.reconfigure(encoding="utf-8", errors="replace")

    json_path = sys.argv[1] if len(sys.argv) > 1 else os.path.join("J:\\", "kursa4", "query.json")

    if not os.path.exists(json_path):
        print(f"❌ Файл не знайдено: {json_path}", file=sys.stderr)
        sys.exit(1)

    env_path = os.path.join(os.path.dirname(__file__), "..", ".env")
    load_dotenv(dotenv_path=env_path)
    db_url = os.getenv("DATABASE_URL")
    if not db_url:
        print("❌ DATABASE_URL не знайдено у .env", file=sys.stderr)
        sys.exit(1)

    print(f"Читаю {json_path}...")
    with open(json_path, encoding="utf-8") as f:
        data = json.load(f)

    bindings = data["results"]["bindings"]
    print(f"  Рядків у файлі: {len(bindings)}")

    movies = parse_movies(bindings)
    print(f"  Унікальних фільмів: {len(movies)}")

    conn = psycopg2.connect(build_dsn(db_url))
    conn.autocommit = False
    cur = conn.cursor()

    inserted = 0
    skipped = 0
    errors = 0

    print("\nЗаписую у базу даних...")
    for i, movie in enumerate(movies):
        try:
            if movie_exists(cur, movie["wikidata_id"], movie["title"], movie["release_year"]):
                skipped += 1
                continue

            item_id = insert_movie(cur, movie)

            if item_id is None:
                skipped += 1
                continue

            for genre_name in movie["genres"]:
                genre_id = ensure_genre(cur, genre_name)
                cur.execute(
                    "INSERT INTO item_genres (item_id, genre_id) VALUES (%s, %s) ON CONFLICT DO NOTHING",
                    (item_id, genre_id),
                )

            inserted += 1

        except Exception as exc:
            conn.rollback()
            errors += 1
            title_safe = movie.get("title", "?")
            print(f"  ⚠️  Пропущено '{title_safe}': {exc}")
            continue

        if inserted % 100 == 0 and inserted > 0:
            conn.commit()
            print(f"  {i + 1}/{len(movies)} оброблено, додано {inserted}...")

    conn.commit()
    cur.close()
    conn.close()

    print(f"\n✅ Готово!")
    print(f"  Додано фільмів:         {inserted}")
    print(f"  Пропущено (дублікати):  {skipped}")
    print(f"  Помилок:                {errors}")


if __name__ == "__main__":
    main()
