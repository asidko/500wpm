/**
 * 500 WPM Speed Reader - Main Application
 * Core reading logic with advanced tokenization and speed progression
 */

// ============================================================================
// CONFIGURATION
// ============================================================================

var API_BASE_URL = (function() {
  // Use current origin for API calls
  var loc = window.location;
  return loc.protocol + '//' + loc.host;
})();

// ============================================================================
// TOKENIZATION & TEXT PROCESSING
// ============================================================================

/**
 * Advanced tokenization: keeps URLs, emails, numbers, hyphenated words as single tokens
 */
function tokenizeText(text) {
  if (!text || !text.trim()) {
    return [];
  }

  // Clean and normalize text
  text = text.trim().replace(/\r\n/g, '\n').replace(/\r/g, '\n');

  // Advanced patterns to preserve as single words
  var urlPattern = /https?:\/\/[^\s]+/g;
  var emailPattern = /[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}/g;
  var phonePattern = /(\+?\d{1,3}[-.\s]?)?\(?\d{3}\)?[-.\s]?\d{3}[-.\s]?\d{4}/g;
  var numberPattern = /[$€£¥]?\d{1,3}(,\d{3})*(\.\d+)?[%€£¥]?/g;

  // Replace special patterns with placeholders
  var placeholders = [];
  var placeholderIndex = 0;

  text = text.replace(urlPattern, function(match) {
    var placeholder = '__URL_' + placeholderIndex + '__';
    placeholders.push({ placeholder: placeholder, original: match });
    placeholderIndex++;
    return placeholder;
  });

  text = text.replace(emailPattern, function(match) {
    var placeholder = '__EMAIL_' + placeholderIndex + '__';
    placeholders.push({ placeholder: placeholder, original: match });
    placeholderIndex++;
    return placeholder;
  });

  text = text.replace(phonePattern, function(match) {
    var placeholder = '__PHONE_' + placeholderIndex + '__';
    placeholders.push({ placeholder: placeholder, original: match });
    placeholderIndex++;
    return placeholder;
  });

  text = text.replace(numberPattern, function(match) {
    var placeholder = '__NUMBER_' + placeholderIndex + '__';
    placeholders.push({ placeholder: placeholder, original: match });
    placeholderIndex++;
    return placeholder;
  });

  // Split on whitespace and em-dashes
  var words = text.split(/\s+|—/);

  // Filter empty strings and restore placeholders
  words = words.filter(function(word) { return word.length > 0; });

  words = words.map(function(word) {
    for (var i = 0; i < placeholders.length; i++) {
      if (word.indexOf(placeholders[i].placeholder) !== -1) {
        return word.replace(placeholders[i].placeholder, placeholders[i].original);
      }
    }
    return word;
  });

  return words;
}

/**
 * Count words in text
 */
function countWords(text) {
  return tokenizeText(text).length;
}

// ============================================================================
// SPEED CALCULATION
// ============================================================================

/**
 * Calculate current speed based on word position
 * @param {number} wordIndex - Current word index (0-based)
 * @param {number} targetSpeed - Target WPM (default 500)
 * @returns {number} Current WPM
 */
function getCurrentSpeed(wordIndex, targetSpeed) {
  if (wordIndex < 10) {
    // Warm-up period: ramp from 100 to targetSpeed over first 10 words
    return 100 + ((targetSpeed - 100) * (wordIndex / 10));
  }
  // Steady reading: maintain target speed
  return targetSpeed;
}

/**
 * Convert WPM to milliseconds per word
 */
function getIntervalMs(wpm) {
  return 60000 / wpm;
}

// ============================================================================
// TIME FORMATTING
// ============================================================================

function formatTime(ms) {
  var seconds = Math.floor(ms / 1000);
  var minutes = Math.floor(seconds / 60);
  var secs = seconds % 60;
  return minutes + 'm ' + secs + 's';
}

// ============================================================================
// CODE EXTRACTION
// ============================================================================

/**
 * Extract the last continuous sequence of digits (6+) from any input
 * Handles: "847291", "https://site.com/847291", "code is 847291", etc.
 */
function extractCodeFromInput(input) {
  if (!input) return '';

  var lastSequence = '';
  var currentSequence = '';

  for (var i = 0; i < input.length; i++) {
    var c = input.charAt(i);
    if (c >= '0' && c <= '9') {
      currentSequence += c;
    } else {
      if (currentSequence.length >= 6) {
        lastSequence = currentSequence;
      }
      currentSequence = '';
    }
  }

  // Check final sequence
  if (currentSequence.length >= 6) {
    lastSequence = currentSequence;
  }

  return lastSequence;
}

// ============================================================================
// SERVER DETECTION
// ============================================================================

var serverAvailable = false;

function checkServerHealth(callback) {
  var xhr = new XMLHttpRequest();
  xhr.open('GET', API_BASE_URL + '/health', true);
  xhr.timeout = 3000; // 3 second timeout for health check

  xhr.onload = function() {
    serverAvailable = xhr.status === 200;
    if (callback) callback(serverAvailable);
  };

  xhr.onerror = function() {
    serverAvailable = false;
    if (callback) callback(false);
  };

  xhr.ontimeout = function() {
    serverAvailable = false;
    if (callback) callback(false);
  };

  xhr.send();
}

// ============================================================================
// HOME PAGE CONTROLLER
// ============================================================================

function initHomePage() {
  // Check if we're on a direct code route (e.g., /847291)
  var pathCode = extractCodeFromInput(window.location.pathname);
  if (pathCode) {
    fetchBookByCode(pathCode);
    return;
  }

  var modeTabs = document.querySelectorAll('.mode-tab');
  var inputModes = document.querySelectorAll('.input-mode');
  var startButton = document.getElementById('startButton');
  var fileInput = document.getElementById('fileInput');
  var fileInfo = document.getElementById('fileInfo');
  var errorMessage = document.getElementById('errorMessage');
  var loading = document.getElementById('loading');
  var successMessage = document.getElementById('successMessage');
  var bookCode = document.getElementById('bookCode');
  var bookLink = document.getElementById('bookLink');
  var startReadingBtn = document.getElementById('startReadingBtn');
  var serverWarning = document.getElementById('serverWarning');
  var codeTab = null;

  // Start reading button click handler
  addEvent(startReadingBtn, 'click', function() {
    window.location.href = 'reader.html';
  });

  // Find the code tab
  for (var i = 0; i < modeTabs.length; i++) {
    if (modeTabs[i].getAttribute('data-mode') === 'code') {
      codeTab = modeTabs[i];
      break;
    }
  }

  var currentMode = 'paste';
  var selectedFile = null;

  // Check server availability on load
  checkServerHealth(function(available) {
    updateServerUI(available);
  });

  function updateServerUI(available) {
    if (!available && serverWarning) {
      serverWarning.className = 'server-warning visible';
      // Disable code tab when server unavailable
      if (codeTab) {
        codeTab.className = 'mode-tab disabled';
        codeTab.setAttribute('title', 'Requires server connection');
      }
    } else if (serverWarning) {
      serverWarning.className = 'server-warning';
      if (codeTab) {
        codeTab.className = 'mode-tab';
        codeTab.removeAttribute('title');
      }
    }
  }

  // Mode tab switching
  for (var i = 0; i < modeTabs.length; i++) {
    addEvent(modeTabs[i], 'click', function() {
      var mode = this.getAttribute('data-mode');

      // Prevent switching to code tab when server unavailable
      if (mode === 'code' && !serverAvailable) {
        showError('Share codes require a server connection. Use paste or file upload instead.');
        return;
      }

      currentMode = mode;

      // Update tab styles (preserve disabled state)
      for (var j = 0; j < modeTabs.length; j++) {
        var tab = modeTabs[j];
        var isDisabled = tab.className.indexOf('disabled') !== -1;
        tab.className = isDisabled ? 'mode-tab disabled' : 'mode-tab';
      }
      this.className = this.className.indexOf('disabled') !== -1 ? 'mode-tab disabled active' : 'mode-tab active';

      // Update input areas
      for (var j = 0; j < inputModes.length; j++) {
        inputModes[j].className = 'input-mode';
      }
      document.getElementById(mode + 'Mode').className = 'input-mode active';

      // Clear messages
      hideError();
      hideSuccess();
    });
  }

  // File input handling
  addEvent(fileInput, 'change', function() {
    if (this.files && this.files.length > 0) {
      selectedFile = this.files[0];
      var fileName = selectedFile.name;
      var fileSize = (selectedFile.size / 1024).toFixed(2);
      fileInfo.textContent = fileName + ' (' + fileSize + ' KB)';
    } else {
      selectedFile = null;
      fileInfo.textContent = 'No file selected';
    }
  });

  // Start button
  addEvent(startButton, 'click', function() {
    hideError();
    hideSuccess();

    if (currentMode === 'paste') {
      handlePasteMode();
    } else if (currentMode === 'upload') {
      handleUploadMode();
    } else if (currentMode === 'code') {
      handleCodeMode();
    }
  });

  function handlePasteMode() {
    var text = document.getElementById('pasteText').value;
    if (!text || !text.trim()) {
      showError('Please paste some text to read.');
      return;
    }

    var words = tokenizeText(text);
    if (words.length === 0) {
      showError('No readable text found.');
      return;
    }

    // Store directly in localStorage and go to reader (no server needed)
    var chapters = [{
      title: 'Pasted Text',
      text: text,
      word_count: words.length
    }];
    storeReadingData(null, chapters);
    window.location.href = 'reader.html';
  }

  function handleUploadMode() {
    if (!selectedFile) {
      showError('Please select a file to upload.');
      return;
    }

    var fileName = selectedFile.name.toLowerCase();

    // Check if it's a .txt file (can be parsed offline)
    if (fileName.indexOf('.txt', fileName.length - 4) !== -1) {
      parseTextFileOffline(selectedFile);
      return;
    }

    // Check for epub/mobi formats (require server)
    var serverFormats = ['.epub', '.mobi', '.azw', '.azw3', '.prc'];
    var needsServer = false;
    for (var i = 0; i < serverFormats.length; i++) {
      if (fileName.indexOf(serverFormats[i], fileName.length - serverFormats[i].length) !== -1) {
        needsServer = true;
        break;
      }
    }

    if (needsServer) {
      // Try backend, with graceful fallback message
      uploadFileToBackend(selectedFile);
      return;
    }

    showError('Unsupported file format. Use .txt for offline reading, or .epub/.mobi with server.');
  }

  function parseTextFileOffline(file) {
    // Check if FileReader is available (may not be on very old Kindle browsers)
    if (typeof FileReader === 'undefined') {
      showError('Your browser does not support file reading. Please paste text instead.');
      return;
    }

    showLoading();

    var reader = new FileReader();
    reader.onload = function(e) {
      hideLoading();
      var text = e.target.result;

      if (!text || !text.trim()) {
        showError('File is empty.');
        return;
      }

      var words = tokenizeText(text);
      if (words.length === 0) {
        showError('No readable text found in file.');
        return;
      }

      // Extract filename without extension for title
      var title = file.name.replace(/\.txt$/i, '') || 'Uploaded Text';

      var chapters = [{
        title: title,
        text: text,
        word_count: words.length
      }];
      storeReadingData(null, chapters);
      window.location.href = 'reader.html';
    };

    reader.onerror = function() {
      hideLoading();
      showError('Failed to read file.');
    };

    reader.readAsText(file);
  }

  function handleCodeMode() {
    var input = document.getElementById('codeInput').value.trim();
    if (!input) {
      showError('Please enter a code or link.');
      return;
    }

    var code = extractCodeFromInput(input);
    if (!code) {
      showError('No valid code found. Please enter a 6-digit numeric code.');
      return;
    }

    fetchBookByCode(code);
  }

  function uploadFileToBackend(file) {
    showLoading();

    var formData = new FormData();
    formData.append('file', file);

    var xhr = new XMLHttpRequest();
    xhr.open('POST', API_BASE_URL + '/api/upload', true);
    xhr.timeout = 30000; // 30 second timeout

    xhr.onload = function() {
      hideLoading();

      if (xhr.status === 200) {
        try {
          var response = JSON.parse(xhr.responseText);
          if (response.code && response.book) {
            showSuccessMessage(response.code);
            storeReadingData(response.code, convertBookToChapters(response.book));
            // User clicks "Start Reading" button manually
          } else if (response.error) {
            showError(response.error + (response.details ? ': ' + response.details : ''));
          } else {
            showError('Failed to process file.');
          }
        } catch (e) {
          showError('Invalid response from server.');
        }
      } else if (xhr.status === 413) {
        showError('File too large. Maximum size is 50MB.');
      } else if (xhr.status === 429) {
        showError('Too many requests. Please wait a moment.');
      } else {
        try {
          var errResponse = JSON.parse(xhr.responseText);
          showError(errResponse.error || 'Server error.');
        } catch (e) {
          showError('Server error. Please try again.');
        }
      }
    };

    xhr.onerror = function() {
      hideLoading();
      showError('Server unavailable. EPUB/MOBI files require a backend server. Try pasting text or uploading a .txt file instead.');
    };

    xhr.ontimeout = function() {
      hideLoading();
      showError('Request timed out. Please try again.');
    };

    xhr.send(formData);
  }

  function uploadTextToBackend(text) {
    showLoading();

    var xhr = new XMLHttpRequest();
    xhr.open('POST', API_BASE_URL + '/api/text', true);
    xhr.setRequestHeader('Content-Type', 'application/json');
    xhr.timeout = 30000; // 30 second timeout

    xhr.onload = function() {
      hideLoading();

      if (xhr.status === 200) {
        try {
          var response = JSON.parse(xhr.responseText);
          if (response.code && response.book) {
            showSuccessMessage(response.code);
            storeReadingData(response.code, convertBookToChapters(response.book));
            // User clicks "Start Reading" button manually
          } else if (response.error) {
            showError(response.error);
          } else {
            showError('Failed to process text.');
          }
        } catch (e) {
          showError('Invalid response from server.');
        }
      } else if (xhr.status === 429) {
        showError('Too many requests. Please wait a moment.');
      } else {
        showError('Server error. Please try again.');
      }
    };

    xhr.onerror = function() {
      hideLoading();
      showError('Cannot connect to server. Please ensure it is running.');
    };

    xhr.ontimeout = function() {
      hideLoading();
      showError('Request timed out. Please try again.');
    };

    xhr.send(JSON.stringify({ text: text }));
  }

  function fetchBookByCode(code) {
    showLoading();

    var xhr = new XMLHttpRequest();
    xhr.open('GET', API_BASE_URL + '/api/book/' + code, true);
    xhr.timeout = 30000; // 30 second timeout

    xhr.onload = function() {
      hideLoading();

      if (xhr.status === 200) {
        try {
          var response = JSON.parse(xhr.responseText);
          if (response.book) {
            storeReadingData(code, convertBookToChapters(response.book));
            window.location.href = 'reader.html';
          } else {
            showError('Book not found.');
          }
        } catch (e) {
          showError('Invalid response from server.');
        }
      } else if (xhr.status === 404) {
        showError('Book not found. Please check the code and try again.');
      } else if (xhr.status === 429) {
        showError('Too many requests. Please wait a moment.');
      } else {
        showError('Server error. Please try again.');
      }
    };

    xhr.onerror = function() {
      hideLoading();
      showError('Server unavailable. Share codes require a backend server. Try pasting text or uploading a .txt file instead.');
    };

    xhr.ontimeout = function() {
      hideLoading();
      showError('Request timed out. Please try again.');
    };

    xhr.send();
  }

  function convertBookToChapters(book) {
    // Convert from API format to reader format
    var chapters = [];
    if (book && book.chapters) {
      for (var i = 0; i < book.chapters.length; i++) {
        var ch = book.chapters[i];
        chapters.push({
          title: ch.title || ('Chapter ' + (i + 1)),
          text: ch.content || '',
          word_count: ch.word_count || 0
        });
      }
    }
    return chapters;
  }

  function showSuccessMessage(code) {
    var link = window.location.origin + '/' + code;
    bookCode.textContent = code;
    bookLink.textContent = link;
    bookLink.href = link;
    successMessage.className = 'success-message visible';
  }

  function hideSuccess() {
    successMessage.className = 'success-message';
  }

  function showError(message) {
    errorMessage.textContent = message;
    errorMessage.className = 'error-message visible';
  }

  function hideError() {
    errorMessage.className = 'error-message';
  }

  function showLoading() {
    loading.className = 'loading visible';
    startButton.disabled = true;
  }

  function hideLoading() {
    loading.className = 'loading';
    startButton.disabled = false;
  }

  function storeReadingData(code, chapters) {
    try {
      localStorage.setItem('readingData', JSON.stringify({
        code: code,
        chapters: chapters
      }));
    } catch (e) {
      // Fallback for browsers without localStorage
      window.readingData = { code: code, chapters: chapters };
    }
  }
}

// ============================================================================
// READER PAGE CONTROLLER
// ============================================================================

function initReaderPage() {
  // Load reading data
  var bookCode = null;
  var chapters = null;
  try {
    var stored = localStorage.getItem('readingData');
    if (stored) {
      var data = JSON.parse(stored);
      // Handle both new format {code, chapters} and legacy format [chapters]
      if (data.chapters) {
        bookCode = data.code || null;
        chapters = data.chapters;
      } else if (Array.isArray(data)) {
        chapters = data;
      }
    }
  } catch (e) {
    var fallback = window.readingData || null;
    if (fallback && fallback.chapters) {
      bookCode = fallback.code || null;
      chapters = fallback.chapters;
    } else if (Array.isArray(fallback)) {
      chapters = fallback;
    }
  }

  if (!chapters || chapters.length === 0) {
    alert('No reading data found. Returning to home page.');
    window.location.href = '/';
    return;
  }

  // Load saved progress for this book
  var savedProgress = null;
  if (bookCode) {
    try {
      var progressKey = 'progress_' + bookCode;
      var progressData = localStorage.getItem(progressKey);
      if (progressData) {
        savedProgress = JSON.parse(progressData);
      }
    } catch (e) {
      // Ignore progress load errors
    }
  }

  // Reader state
  var state = {
    bookCode: bookCode,
    chapters: chapters,
    currentChapterIndex: savedProgress ? savedProgress.chapterIndex : 0,
    allWords: [],
    currentWordIndex: savedProgress ? savedProgress.wordIndex : 0,
    targetSpeed: savedProgress ? savedProgress.speed : 500,
    isPlaying: false,
    timer: null,
    startTime: null,
    totalElapsedTime: savedProgress ? savedProgress.elapsedTime : 0
  };

  // Prepare all words with chapter boundaries
  prepareWords();

  // DOM elements
  var readerContainer = document.getElementById('readerContainer');
  var wordDisplay = document.getElementById('wordDisplay');
  var speedDisplay = document.getElementById('speedDisplay');
  var progressDisplay = document.getElementById('progressDisplay');
  var chapterInfo = document.getElementById('chapterInfo');
  var chapterList = document.getElementById('chapterList');
  var chapterItems = document.getElementById('chapterItems');
  var playPauseButton = document.getElementById('playPauseButton');
  var increaseSpeed = document.getElementById('increaseSpeed');
  var decreaseSpeed = document.getElementById('decreaseSpeed');
  var restartButton = document.getElementById('restartButton');
  var backButton = document.getElementById('backButton');
  var forwardButton = document.getElementById('forwardButton');
  var closeButton = document.getElementById('closeButton');
  var chapterCompleteScreen = document.getElementById('chapterCompleteScreen');
  var readingCompleteScreen = document.getElementById('readingCompleteScreen');
  var nextChapterButton = document.getElementById('nextChapterButton');
  var chaptersButton = document.getElementById('chaptersButton');
  var homeFromChapter = document.getElementById('homeFromChapter');
  var restartFromComplete = document.getElementById('restartFromComplete');
  var homeFromComplete = document.getElementById('homeFromComplete');
  var chapterCompleteTitle = document.getElementById('chapterCompleteTitle');
  var completionMessage = document.getElementById('completionMessage');
  var totalWordsInline = document.getElementById('totalWordsInline');
  var timeTakenInline = document.getElementById('timeTakenInline');

  // Initialize UI
  updateSpeedDisplay();
  updateProgressDisplay();
  renderChapterList();

  // Show correct word if resuming from saved progress
  if (state.currentWordIndex > 0 && state.currentWordIndex < state.allWords.length) {
    wordDisplay.textContent = state.allWords[state.currentWordIndex].word;
    state.currentChapterIndex = state.allWords[state.currentWordIndex].chapterIndex;
  }

  pause();

  // Event listeners
  addEvent(playPauseButton, 'click', togglePlayPause);
  addEvent(increaseSpeed, 'click', function() { adjustSpeed(50); });
  addEvent(decreaseSpeed, 'click', function() { adjustSpeed(-50); });
  addEvent(restartButton, 'click', restart);
  addEvent(backButton, 'click', function() { skipWords(-10); });
  addEvent(forwardButton, 'click', function() { skipWords(10); });
  addEvent(closeButton, 'click', goHome);
  addEvent(nextChapterButton, 'click', nextChapter);
  addEvent(chaptersButton, 'click', showChapterList);
  addEvent(homeFromChapter, 'click', goHome);
  addEvent(restartFromComplete, 'click', restart);
  addEvent(homeFromComplete, 'click', goHome);

  // Click anywhere to pause/resume
  addEvent(readerContainer, 'click', function(e) {
    // Don't toggle if clicking on a button
    if (e.target.tagName === 'BUTTON') {
      return;
    }
    togglePlayPause();
  });

  // Keyboard shortcuts
  addEvent(document, 'keydown', function(e) {
    var key = e.keyCode || e.which;

    // Spacebar: play/pause
    if (key === 32) {
      e.preventDefault();
      togglePlayPause();
    }
    // Up arrow: increase speed
    else if (key === 38) {
      e.preventDefault();
      adjustSpeed(50);
    }
    // Down arrow: decrease speed
    else if (key === 40) {
      e.preventDefault();
      adjustSpeed(-50);
    }
    // Left arrow: go back 10 words
    else if (key === 37) {
      e.preventDefault();
      skipWords(-10);
    }
    // Right arrow: skip forward 10 words
    else if (key === 39) {
      e.preventDefault();
      skipWords(10);
    }
    // R: restart
    else if (key === 82 || key === 114) {
      e.preventDefault();
      restart();
    }
    // Escape: go home
    else if (key === 27) {
      e.preventDefault();
      goHome();
    }
  });

  // Core functions
  function prepareWords() {
    state.allWords = [];
    for (var i = 0; i < state.chapters.length; i++) {
      var chapter = state.chapters[i];
      var words = tokenizeText(chapter.text);
      for (var j = 0; j < words.length; j++) {
        state.allWords.push({
          word: words[j],
          chapterIndex: i,
          wordIndexInChapter: j,
          isLastInChapter: j === words.length - 1
        });
      }
    }
  }

  function play() {
    if (state.currentWordIndex >= state.allWords.length) {
      showReadingComplete();
      return;
    }

    state.isPlaying = true;
    state.startTime = Date.now();
    readerContainer.className = 'reader-container reader playing';
    playPauseButton.innerHTML = '&#10074;&#10074; Pause';

    displayNextWord();
  }

  function pause() {
    state.isPlaying = false;
    if (state.timer) {
      clearTimeout(state.timer);
      state.timer = null;
    }
    if (state.startTime) {
      state.totalElapsedTime += Date.now() - state.startTime;
      state.startTime = null;
    }
    readerContainer.className = 'reader-container reader paused';
    playPauseButton.innerHTML = '&#9654; Play';
    updateChapterInfo();
    saveProgress();
  }

  function saveProgress() {
    if (!state.bookCode) return;
    try {
      var progressKey = 'progress_' + state.bookCode;
      localStorage.setItem(progressKey, JSON.stringify({
        wordIndex: state.currentWordIndex,
        chapterIndex: state.currentChapterIndex,
        speed: state.targetSpeed,
        elapsedTime: state.totalElapsedTime
      }));
    } catch (e) {
      // Ignore save errors (quota exceeded, etc.)
    }
  }

  function clearProgress() {
    if (!state.bookCode) return;
    try {
      var progressKey = 'progress_' + state.bookCode;
      localStorage.removeItem(progressKey);
    } catch (e) {
      // Ignore clear errors
    }
  }

  function togglePlayPause() {
    if (state.isPlaying) {
      pause();
    } else {
      play();
    }
  }

  function displayNextWord() {
    if (!state.isPlaying || state.currentWordIndex >= state.allWords.length) {
      return;
    }

    var wordData = state.allWords[state.currentWordIndex];
    wordDisplay.textContent = wordData.word;

    // Check if chapter changed
    if (wordData.chapterIndex !== state.currentChapterIndex) {
      state.currentChapterIndex = wordData.chapterIndex;
    }

    // Update progress
    updateProgressDisplay();

    // Check if last word in chapter
    if (wordData.isLastInChapter && state.currentChapterIndex < state.chapters.length - 1) {
      // Chapter complete
      state.currentWordIndex++;
      pause();
      showChapterComplete();
      return;
    }

    // Move to next word
    state.currentWordIndex++;

    // Calculate speed and interval
    var currentSpeed = getCurrentSpeed(state.currentWordIndex - 1, state.targetSpeed);
    var interval = getIntervalMs(currentSpeed);

    // Schedule next word
    state.timer = setTimeout(displayNextWord, interval);

    // Check if reading complete
    if (state.currentWordIndex >= state.allWords.length) {
      pause();
      showReadingComplete();
    }
  }

  function adjustSpeed(delta) {
    state.targetSpeed = Math.max(50, state.targetSpeed + delta);
    updateSpeedDisplay();
  }

  function updateSpeedDisplay() {
    speedDisplay.textContent = state.targetSpeed + ' WPM';
  }

  function updateProgressDisplay() {
    var current = state.currentWordIndex + 1;
    var total = state.allWords.length;
    var percentage = Math.round((current / total) * 100);
    progressDisplay.textContent = percentage + '%';
  }

  function updateChapterInfo() {
    if (state.chapters.length > 1) {
      var chapter = state.chapters[state.currentChapterIndex];
      chapterInfo.textContent = chapter.title;
      chapterInfo.className = 'chapter-info visible';
    }
  }

  function renderChapterList() {
    if (state.chapters.length <= 1) {
      return;
    }

    chapterItems.innerHTML = '';
    for (var i = 0; i < state.chapters.length; i++) {
      var chapter = state.chapters[i];
      var item = document.createElement('div');
      item.className = 'chapter-item';
      if (i === state.currentChapterIndex) {
        item.className = 'chapter-item current';
      }
      item.textContent = chapter.title;
      item.setAttribute('data-index', i);

      addEvent(item, 'click', function() {
        var index = parseInt(this.getAttribute('data-index'));
        jumpToChapter(index);
      });

      chapterItems.appendChild(item);
    }
  }

  function showChapterList() {
    pause();
    if (state.chapters.length > 1) {
      chapterList.className = 'chapter-list visible';
    }
  }

  function hideChapterList() {
    chapterList.className = 'chapter-list';
  }

  function jumpToChapter(chapterIndex) {
    hideChapterList();

    // Find first word of chapter
    for (var i = 0; i < state.allWords.length; i++) {
      if (state.allWords[i].chapterIndex === chapterIndex) {
        state.currentWordIndex = i;
        state.currentChapterIndex = chapterIndex;
        break;
      }
    }

    updateProgressDisplay();
    updateChapterInfo();
    renderChapterList();
    wordDisplay.textContent = state.allWords[state.currentWordIndex].word;
  }

  function skipWords(count) {
    var wasPlaying = state.isPlaying;
    if (wasPlaying) {
      pause();
    }

    state.currentWordIndex = Math.max(0, Math.min(
      state.allWords.length - 1,
      state.currentWordIndex + count
    ));

    // Update current chapter
    if (state.currentWordIndex < state.allWords.length) {
      state.currentChapterIndex = state.allWords[state.currentWordIndex].chapterIndex;
    }

    updateProgressDisplay();
    updateChapterInfo();
    renderChapterList();
    wordDisplay.textContent = state.allWords[state.currentWordIndex].word;

    if (wasPlaying) {
      play();
    }
  }

  function restart() {
    hideChapterList();
    chapterCompleteScreen.className = 'completion-screen';
    readingCompleteScreen.className = 'completion-screen';
    completionMessage.className = 'completion-message';

    state.currentWordIndex = 0;
    state.currentChapterIndex = 0;
    state.totalElapsedTime = 0;
    state.startTime = null;

    // Re-enable play and forward buttons
    playPauseButton.disabled = false;
    forwardButton.disabled = false;

    updateProgressDisplay();
    updateChapterInfo();
    renderChapterList();
    wordDisplay.textContent = state.allWords[0].word;

    pause();
  }

  function showChapterComplete() {
    var chapter = state.chapters[state.currentChapterIndex];
    chapterCompleteTitle.textContent = '✓ ' + chapter.title + ' - Complete!';
    chapterCompleteScreen.className = 'completion-screen visible';
  }

  function nextChapter() {
    chapterCompleteScreen.className = 'completion-screen';

    if (state.currentChapterIndex < state.chapters.length - 1) {
      jumpToChapter(state.currentChapterIndex + 1);
      play();
    } else {
      showReadingComplete();
    }
  }

  function showReadingComplete() {
    pause();
    clearProgress(); // Book finished, clear saved progress

    var totalWords = state.allWords.length;
    var timeTaken = state.totalElapsedTime + (state.startTime ? Date.now() - state.startTime : 0);

    // Update inline completion message
    totalWordsInline.textContent = totalWords.toLocaleString();
    timeTakenInline.textContent = formatTime(timeTaken);
    completionMessage.className = 'completion-message visible';

    // Update old overlay for compatibility (if still needed)
    document.getElementById('totalWords').textContent = totalWords.toLocaleString();
    document.getElementById('timeTaken').textContent = formatTime(timeTaken);

    // Disable play and forward buttons
    playPauseButton.disabled = true;
    forwardButton.disabled = true;

    // Keep back button enabled for navigation
    wordDisplay.textContent = state.allWords[state.allWords.length - 1].word;
  }

  function goHome() {
    // Clear reading data
    try {
      localStorage.removeItem('readingData');
    } catch (e) {
      window.readingData = null;
    }
    window.location.href = '/';
  }
}

// ============================================================================
// INITIALIZATION
// ============================================================================

addEvent(window, 'load', function() {
  // Detect which page we're on
  if (document.getElementById('startButton')) {
    initHomePage();
  } else if (document.getElementById('readerContainer')) {
    initReaderPage();
  }
});
