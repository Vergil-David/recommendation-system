#!/usr/bin/env python3
"""
Fill missing image_url for movies (via TMDB) and books (via Open Library).

Requirements:
    pip install requests psycopg2-binary python-dotenv
"""

import os
import sys
import time
import urllib.parse
import requests
import psycopg2
import psycopg2.extras
from dotenv import load_dotenv

# ---------------------------------------------------------------------------
# Config
# ---------------------------------------------------------------------------

TMDB_BASE     = "https://api.themoviedb.org/3"
TMDB_IMG_BASE = "https://image.tmdb.org/t/p/w500"
OL_COVER_BASE = "https://covers.openlibrary.org/b/title/{title}-L.jpg?default=false"

REQUEST_DELAY = 0.25   # seconds between TMDB calls (rate limit: 40 req/10s)


# ---------------------------------------------------------------------------
# TMDB helpers
# ---------------------------------------------------------------------------

def tmdb_search(title: str, year: int | None, api_key: str) -> str | None:
    """Return poster URL for the best-matching movie, or None."""
    params = {
        "api_key": api_key,
        "query": title,
        "language": "uk-UA",
        "include_adult": "false",
    }
    if year:
        params["primary_release_year"] = year

    try:
        resp = requests.get(f"{TMDB_BASE}/search/movie", params=params, timeout=10)
        resp.raise_for_status()
        results = resp.json().get("results", [])
    except Exception:
        return None

    # Try exact year match first, then fallback to first result
    for result in results:
        poster = result.get("poster_path", "")
        if poster:
            return TMDB_IMG_BASE + poster

    return None


# ---------------------------------------------------------------------------
# Open Library helpers
# ---------------------------------------------------------------------------

def ol_cover(title: str) -> str | None:
    """Search Open Library by title, return cover URL if a cover exists."""
    try:
        resp = requests.get(
            "https://openlibrary.org/search.json",
            params={"title": title, "limit": "3", "fields": "cover_i,title"},
            timeout=10,
        )
        resp.raise_for_status()
        docs = resp.json().get("docs", [])
        for doc in docs:
            cover_id = doc.get("cover_i")
            if cover_id:
                return f"https://covers.openlibrary.org/b/id/{cover_id}-L.jpg"
    except Exception:
        pass
    return None


def google_books_cover(title: str) -> str | None:
    """Search Google Books by title, return thumbnail URL if found."""
    try:
        resp = requests.get(
            "https://www.googleapis.com/books/v1/volumes",
            params={"q": f"intitle:{title}", "maxResults": "3", "langRestrict": "uk"},
            timeout=10,
        )
        resp.raise_for_status()
        items = resp.json().get("items", [])
        for item in items:
            links = item.get("volumeInfo", {}).get("imageLinks", {})
            thumb = links.get("thumbnail") or links.get("smallThumbnail")
            if thumb:
                # Use larger size
                thumb = thumb.replace("zoom=1", "zoom=2").replace("http://", "https://")
                return thumb
    except Exception:
        pass
    return None


# ---------------------------------------------------------------------------
# DB helpers
# ---------------------------------------------------------------------------

def build_dsn(raw_url: str) -> str:
    parsed = urllib.parse.urlparse(raw_url)
    params = urllib.parse.parse_qs(parsed.query)
    allowed = {"sslmode", "sslcert", "sslkey", "sslrootcert", "connect_timeout"}
    clean = {k: v[0] for k, v in params.items() if k in allowed}
    return parsed._replace(query=urllib.parse.urlencode(clean)).geturl()


def get_items_without_images(cur, item_type: str) -> list[dict]:
    cur.execute(
        """
        SELECT id, title, release_year
        FROM items
        WHERE type = %s
          AND (image_url IS NULL OR image_url = '')
        ORDER BY id
        """,
        (item_type,),
    )
    return [{"id": r[0], "title": r[1], "year": r[2]} for r in cur.fetchall()]


def update_image(cur, item_id: int, image_url: str):
    cur.execute(
        "UPDATE items SET image_url = %s WHERE id = %s",
        (image_url, item_id),
    )


def reconnect(dsn: str):
    conn = psycopg2.connect(dsn)
    conn.autocommit = False
    return conn, conn.cursor()


# ---------------------------------------------------------------------------
# Main
# ---------------------------------------------------------------------------

def main():
    if hasattr(sys.stdout, "reconfigure"):
        sys.stdout.reconfigure(encoding="utf-8", errors="replace")
    if hasattr(sys.stderr, "reconfigure"):
        sys.stderr.reconfigure(encoding="utf-8", errors="replace")

    env_path = os.path.join(os.path.dirname(__file__), "..", ".env")
    load_dotenv(dotenv_path=env_path)

    db_url = os.getenv("DATABASE_URL")
    tmdb_key = os.getenv("TMDB_API_KEY")

    if not db_url:
        print("❌ DATABASE_URL не знайдено", file=sys.stderr)
        sys.exit(1)
    if not tmdb_key:
        print("❌ TMDB_API_KEY не знайдено", file=sys.stderr)
        sys.exit(1)

    conn = psycopg2.connect(build_dsn(db_url))
    conn.autocommit = False
    cur = conn.cursor()

    # ---- MOVIES ----
    movies = get_items_without_images(cur, "movie")
    print(f"Фільмів без зображення: {len(movies)}")

    m_found = m_missing = 0
    for i, movie in enumerate(movies):
        poster = tmdb_search(movie["title"], movie["year"], tmdb_key)
        # Retry without year constraint if not found
        if not poster and movie["year"]:
            poster = tmdb_search(movie["title"], None, tmdb_key)
        if poster:
            update_image(cur, movie["id"], poster)
            m_found += 1
        else:
            m_missing += 1

        if (i + 1) % 50 == 0:
            conn.commit()
            print(f"  [{i+1}/{len(movies)}] знайдено: {m_found}, не знайдено: {m_missing}")

        time.sleep(REQUEST_DELAY)

    conn.commit()
    print(f"Фільми: ✅ знайдено {m_found}, ❌ не знайдено {m_missing}\n")

    # ---- BOOKS ----
    books = get_items_without_images(cur, "book")
    print(f"Книг без зображення: {len(books)}")

    b_found = b_missing = 0
    dsn = build_dsn(db_url)
    for i, book in enumerate(books):
        # Try Open Library first, then Google Books as fallback
        cover = ol_cover(book["title"]) or google_books_cover(book["title"])
        if cover:
            try:
                update_image(cur, book["id"], cover)
            except psycopg2.OperationalError:
                # Reconnect on dropped connection
                try: conn.close()
                except Exception: pass
                conn, cur = reconnect(dsn)
                update_image(cur, book["id"], cover)
            b_found += 1
        else:
            b_missing += 1

        if (i + 1) % 25 == 0:
            try:
                conn.commit()
            except psycopg2.OperationalError:
                conn, cur = reconnect(dsn)
            print(f"  [{i+1}/{len(books)}] знайдено: {b_found}, не знайдено: {b_missing}")

        time.sleep(0.2)

    conn.commit()
    print(f"Книги:  ✅ знайдено {b_found}, ❌ не знайдено {b_missing}\n")

    cur.close()
    conn.close()
    print("✅ Готово!")


if __name__ == "__main__":
    main()
