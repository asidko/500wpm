# Speed Reader

Minimalist web-based speed reading app using RSVP (Rapid Serial Visual Presentation). Read faster by displaying one word at a time.

## Why?

Inspired by this post on X:

<img src="https://github.com/user-attachments/assets/2984295c-dd49-4ec6-8afc-7abb72264641" width="240" height="426" />

An average 250-page book will be read in about **2 hours and 5 minutes** at a reading speed of 500 words per minute.
## Features

- RSVP reading with 10-word warm-up (100→500 WPM)
- Three input modes: paste text, upload files (.txt/.epub), or fetch URLs
- Adjustable speed: 100-1000 WPM (±50 WPM steps)
- Navigation: skip ±10 words
- Keyboard shortcuts: `Space` (play/pause), `↑↓` (speed), `←→` (navigate)
- **Kindle & e-ink friendly** - optimized for e-readers with minimal refresh
- **Old browser support** - works on IE8+, Kindle browsers, legacy devices

## Quick Start

### Docker Compose (Recommended)

```bash
docker-compose up -d
```

### Docker CLI

```bash
docker build -t speed-reader .
docker run -d -p 3000:3000 -p 8000:8000 speed-reader
```

Access at: **http://localhost:3000**

## Manual Setup

```bash
# Frontend
npm install && npm run build

# Backend (in new terminal)
cd backend
python -m venv venv && source venv/bin/activate
pip install -r requirements.txt
python main.py &

# Serve frontend
python -m http.server 3000
```

## Example Usage

Try Project Gutenberg books:
```
https://www.gutenberg.org/cache/epub/1661/pg1661.txt
https://www.gutenberg.org/cache/epub/1342/pg1342.txt
https://www.gutenberg.org/cache/epub/11/pg11.txt
```

## Tech Stack

- **Frontend**: JavaScript (Babel ES5), Kindle-compatible CSS
- **Backend**: FastAPI (Python), EPUB parsing, rate limiting
- **Limits**: 50MB files, 20 req/min, 10s timeout
