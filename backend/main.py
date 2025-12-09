"""
500 WPM Speed Reader - Backend API
FastAPI server for file fetching and EPUB parsing
"""

import re
import ipaddress
from typing import Optional
from urllib.parse import urlparse
import requests
from fastapi import FastAPI, UploadFile, File, HTTPException, Request, Body
from fastapi.middleware.cors import CORSMiddleware
from pydantic import BaseModel
from bs4 import BeautifulSoup
import ebooklib
from ebooklib import epub
from slowapi import Limiter, _rate_limit_exceeded_handler
from slowapi.util import get_remote_address
from slowapi.errors import RateLimitExceeded

# Rate limiter
limiter = Limiter(key_func=get_remote_address)

# FastAPI app
app = FastAPI(title="500 WPM Reader API")

# Add rate limit handler
app.state.limiter = limiter
app.add_exception_handler(RateLimitExceeded, _rate_limit_exceeded_handler)

# CORS middleware
app.add_middleware(
    CORSMiddleware,
    allow_origins=["*"],
    allow_credentials=True,
    allow_methods=["*"],
    allow_headers=["*"],
)

# Constants
MAX_FILE_SIZE = 50 * 1024 * 1024  # 50MB
REQUEST_TIMEOUT = 10  # seconds

# Private IP ranges to block (SSRF protection)
PRIVATE_IP_RANGES = [
    ipaddress.ip_network('127.0.0.0/8'),
    ipaddress.ip_network('10.0.0.0/8'),
    ipaddress.ip_network('172.16.0.0/12'),
    ipaddress.ip_network('192.168.0.0/16'),
    ipaddress.ip_network('169.254.0.0/16'),  # Link-local
    ipaddress.ip_network('::1/128'),  # IPv6 localhost
    ipaddress.ip_network('fe80::/10'),  # IPv6 link-local
]


class URLRequest(BaseModel):
    url: str


class Chapter:
    def __init__(self, title: str, text: str):
        self.title = title
        self.text = text
        self.word_count = len(text.split())


def is_private_ip(hostname: str) -> bool:
    """Check if hostname resolves to a private IP address"""
    try:
        # Handle localhost
        if hostname.lower() in ['localhost', '0.0.0.0']:
            return True

        # Resolve hostname to IP
        import socket
        ip = socket.gethostbyname(hostname)
        ip_obj = ipaddress.ip_address(ip)

        # Check against private ranges
        for private_range in PRIVATE_IP_RANGES:
            if ip_obj in private_range:
                return True

        return False
    except Exception:
        return True  # Block on error for safety


def validate_url(url: str) -> tuple[bool, str]:
    """Validate URL for security"""
    try:
        parsed = urlparse(url)

        # Only allow HTTP/HTTPS
        if parsed.scheme not in ['http', 'https']:
            return False, "Only HTTP and HTTPS protocols are allowed"

        # Check hostname
        if not parsed.hostname:
            return False, "Invalid URL"

        # Block private IPs
        if is_private_ip(parsed.hostname):
            return False, "Private IPs and localhost are not allowed for security reasons"

        return True, ""
    except Exception as e:
        return False, str(e)


def extract_text_from_html(html: str) -> str:
    """Extract clean text from HTML"""
    soup = BeautifulSoup(html, 'html.parser')

    # Remove script and style elements
    for script in soup(["script", "style"]):
        script.decompose()

    # Get text
    text = soup.get_text()

    # Clean up whitespace
    lines = (line.strip() for line in text.splitlines())
    chunks = (phrase.strip() for line in lines for phrase in line.split("  "))
    text = ' '.join(chunk for chunk in chunks if chunk)

    return text


def parse_txt_file(content: bytes) -> list[Chapter]:
    """Parse plain text file"""
    try:
        text = content.decode('utf-8')
    except UnicodeDecodeError:
        try:
            text = content.decode('latin-1')
        except Exception:
            raise HTTPException(400, "Unable to decode text file")

    text = text.strip()
    if not text:
        raise HTTPException(400, "File is empty")

    chapter = Chapter(title="Untitled", text=text)
    return [chapter]


def parse_epub_file(content: bytes) -> list[Chapter]:
    """Parse EPUB file and extract chapters"""
    import io
    import tempfile

    try:
        # Write to temp file (ebooklib requires file path)
        with tempfile.NamedTemporaryFile(delete=False, suffix='.epub') as tmp:
            tmp.write(content)
            tmp_path = tmp.name

        # Parse EPUB
        book = epub.read_epub(tmp_path)

        chapters = []
        chapter_num = 1

        # Extract chapters
        for item in book.get_items():
            if item.get_type() == ebooklib.ITEM_DOCUMENT:
                html_content = item.get_content().decode('utf-8')
                text = extract_text_from_html(html_content)

                # Skip if too short (likely TOC or metadata)
                if len(text.strip()) < 100:
                    continue

                # Try to get title from item
                title = item.get_name()
                if not title or title.endswith('.html') or title.endswith('.xhtml'):
                    title = f"Chapter {chapter_num}"

                chapter = Chapter(title=title, text=text)
                chapters.append(chapter)
                chapter_num += 1

        # Clean up temp file
        import os
        try:
            os.unlink(tmp_path)
        except Exception:
            pass

        if not chapters:
            raise HTTPException(400, "No readable content found in EPUB file")

        return chapters

    except HTTPException:
        raise
    except Exception as e:
        raise HTTPException(400, f"Failed to parse EPUB file: {str(e)}")


def detect_format(filename: str, content_type: Optional[str]) -> str:
    """Detect file format"""
    filename_lower = filename.lower()

    if filename_lower.endswith('.txt'):
        return 'txt'
    elif filename_lower.endswith('.epub'):
        return 'epub'
    elif content_type:
        if 'text/plain' in content_type:
            return 'txt'
        elif 'epub' in content_type:
            return 'epub'

    raise HTTPException(400, "Unsupported file format. Supported: .txt, .epub")


@app.get("/")
def root():
    return {"message": "500 WPM Reader API", "status": "running"}


@app.post("/api/process")
@limiter.limit("20/minute")
async def process_file(
    request: Request,
    file: Optional[UploadFile] = File(None)
):
    """
    Unified endpoint for processing URLs and file uploads
    Returns chaptered text structure
    """

    # Try to parse JSON body for URL mode
    url_request = None
    content_type = request.headers.get('content-type', '')
    if 'application/json' in content_type:
        try:
            body = await request.json()
            if 'url' in body:
                url_request = URLRequest(url=body['url'])
        except Exception:
            pass

    # Handle URL mode
    if url_request and url_request.url:
        url = url_request.url.strip()

        # Validate URL
        is_valid, error_msg = validate_url(url)
        if not is_valid:
            return {
                "success": False,
                "error": "Invalid URL",
                "message": error_msg
            }

        # Fetch file
        try:
            response = requests.get(url, timeout=REQUEST_TIMEOUT, stream=True)
            response.raise_for_status()

            # Check file size
            content_length = response.headers.get('content-length')
            if content_length and int(content_length) > MAX_FILE_SIZE:
                return {
                    "success": False,
                    "error": "File too large",
                    "message": "File exceeds 50MB limit"
                }

            # Read content
            content = b''
            for chunk in response.iter_content(chunk_size=8192):
                content += chunk
                if len(content) > MAX_FILE_SIZE:
                    return {
                        "success": False,
                        "error": "File too large",
                        "message": "File exceeds 50MB limit"
                    }

            # Detect format
            filename = url.split('/')[-1].split('?')[0]
            content_type = response.headers.get('content-type')
            file_format = detect_format(filename, content_type)

        except requests.Timeout:
            return {
                "success": False,
                "error": "Timeout",
                "message": "Request timed out. Please try again."
            }
        except requests.RequestException as e:
            return {
                "success": False,
                "error": "Fetch failed",
                "message": f"Failed to fetch URL: {str(e)}"
            }
        except HTTPException as e:
            return {
                "success": False,
                "error": "Invalid format",
                "message": e.detail
            }

        source = url

    # Handle file upload mode
    elif file:
        # Check file size
        content = await file.read()
        if len(content) > MAX_FILE_SIZE:
            return {
                "success": False,
                "error": "File too large",
                "message": "File exceeds 50MB limit"
            }

        # Detect format
        try:
            file_format = detect_format(file.filename, file.content_type)
        except HTTPException as e:
            return {
                "success": False,
                "error": "Invalid format",
                "message": e.detail
            }

        source = file.filename

    else:
        return {
            "success": False,
            "error": "Missing input",
            "message": "Please provide either a URL or a file upload"
        }

    # Process based on format
    try:
        if file_format == 'txt':
            chapters = parse_txt_file(content)
        elif file_format == 'epub':
            chapters = parse_epub_file(content)
        else:
            return {
                "success": False,
                "error": "Unsupported format",
                "message": f"Format '{file_format}' is not supported"
            }

        # Build response
        total_words = sum(ch.word_count for ch in chapters)

        return {
            "success": True,
            "chapters": [
                {
                    "title": ch.title,
                    "text": ch.text,
                    "word_count": ch.word_count
                }
                for ch in chapters
            ],
            "total_words": total_words,
            "source": source,
            "format": file_format
        }

    except HTTPException as e:
        return {
            "success": False,
            "error": "Parse error",
            "message": e.detail
        }
    except Exception as e:
        return {
            "success": False,
            "error": "Processing error",
            "message": f"Failed to process file: {str(e)}"
        }


if __name__ == "__main__":
    import uvicorn
    print("Starting 500 WPM Reader API on http://localhost:8000")
    uvicorn.run(app, host="0.0.0.0", port=8000)
