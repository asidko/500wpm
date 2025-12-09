/**
 * 500 WPM Speed Reader - Main Application
 * Core reading logic with advanced tokenization and speed progression
 */

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
  const urlPattern = /https?:\/\/[^\s]+/g;
  const emailPattern = /[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}/g;
  const phonePattern = /(\+?\d{1,3}[-.\s]?)?\(?\d{3}\)?[-.\s]?\d{3}[-.\s]?\d{4}/g;
  const numberPattern = /[$€£¥]?\d{1,3}(,\d{3})*(\.\d+)?[%€£¥]?/g;

  // Replace special patterns with placeholders
  const placeholders = [];
  let placeholderIndex = 0;

  text = text.replace(urlPattern, (match) => {
    const placeholder = `__URL_${placeholderIndex}__`;
    placeholders.push({ placeholder, original: match });
    placeholderIndex++;
    return placeholder;
  });

  text = text.replace(emailPattern, (match) => {
    const placeholder = `__EMAIL_${placeholderIndex}__`;
    placeholders.push({ placeholder, original: match });
    placeholderIndex++;
    return placeholder;
  });

  text = text.replace(phonePattern, (match) => {
    const placeholder = `__PHONE_${placeholderIndex}__`;
    placeholders.push({ placeholder, original: match });
    placeholderIndex++;
    return placeholder;
  });

  text = text.replace(numberPattern, (match) => {
    const placeholder = `__NUMBER_${placeholderIndex}__`;
    placeholders.push({ placeholder, original: match });
    placeholderIndex++;
    return placeholder;
  });

  // Split on whitespace and em-dashes
  let words = text.split(/\s+|—/);

  // Filter empty strings and restore placeholders
  words = words.filter((word) => word.length > 0);

  words = words.map((word) => {
    for (let i = 0; i < placeholders.length; i++) {
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
  const seconds = Math.floor(ms / 1000);
  const minutes = Math.floor(seconds / 60);
  const secs = seconds % 60;
  return minutes + 'm ' + secs + 's';
}

// ============================================================================
// HOME PAGE CONTROLLER
// ============================================================================

function initHomePage() {
  const modeTabs = document.querySelectorAll('.mode-tab');
  const inputModes = document.querySelectorAll('.input-mode');
  const startButton = document.getElementById('startButton');
  const fileInput = document.getElementById('fileInput');
  const fileInfo = document.getElementById('fileInfo');
  const errorMessage = document.getElementById('errorMessage');
  const loading = document.getElementById('loading');

  let currentMode = 'paste';
  let selectedFile = null;

  // Mode tab switching
  for (let i = 0; i < modeTabs.length; i++) {
    addEvent(modeTabs[i], 'click', function() {
      const mode = this.getAttribute('data-mode');
      currentMode = mode;

      // Update tab styles
      for (let j = 0; j < modeTabs.length; j++) {
        modeTabs[j].className = 'mode-tab';
      }
      this.className = 'mode-tab active';

      // Update input areas
      for (let j = 0; j < inputModes.length; j++) {
        inputModes[j].className = 'input-mode';
      }
      document.getElementById(mode + 'Mode').className = 'input-mode active';

      // Clear error
      hideError();
    });
  }

  // File input handling
  addEvent(fileInput, 'change', function() {
    if (this.files && this.files.length > 0) {
      selectedFile = this.files[0];
      const fileName = selectedFile.name;
      const fileSize = (selectedFile.size / 1024).toFixed(2);
      fileInfo.textContent = fileName + ' (' + fileSize + ' KB)';
    } else {
      selectedFile = null;
      fileInfo.textContent = 'No file selected';
    }
  });

  // Start button
  addEvent(startButton, 'click', function() {
    hideError();

    if (currentMode === 'paste') {
      handlePasteMode();
    } else if (currentMode === 'upload') {
      handleUploadMode();
    } else if (currentMode === 'url') {
      handleUrlMode();
    }
  });

  function handlePasteMode() {
    const text = document.getElementById('pasteText').value;
    if (!text || !text.trim()) {
      showError('Please paste some text to read.');
      return;
    }

    const words = tokenizeText(text);
    if (words.length === 0) {
      showError('No readable text found.');
      return;
    }

    // Store data and navigate to reader
    const chapters = [{
      title: 'Untitled',
      text: text,
      word_count: words.length
    }];
    storeReadingData(chapters);
    window.location.href = 'reader.html';
  }

  function handleUploadMode() {
    if (!selectedFile) {
      showError('Please select a file to upload.');
      return;
    }

    const fileName = selectedFile.name.toLowerCase();

    if (fileName.endsWith('.txt')) {
      // Read .txt file in browser
      const reader = new FileReader();
      reader.onload = function(e) {
        const text = e.target.result;
        const words = tokenizeText(text);

        if (words.length === 0) {
          showError('No readable text found in file.');
          return;
        }

        const chapters = [{
          title: selectedFile.name.replace('.txt', ''),
          text: text,
          word_count: words.length
        }];
        storeReadingData(chapters);
        window.location.href = 'reader.html';
      };
      reader.onerror = function() {
        showError('Failed to read file. Please try again.');
      };
      reader.readAsText(selectedFile);

    } else if (fileName.endsWith('.epub')) {
      // Upload to backend for parsing
      uploadToBackend(selectedFile);

    } else {
      showError('Unsupported file format. Please use .txt or .epub files.');
    }
  }

  function handleUrlMode() {
    const url = document.getElementById('urlInput').value.trim();
    if (!url) {
      showError('Please enter a URL.');
      return;
    }

    if (!url.match(/^https?:\/\//)) {
      showError('Please enter a valid URL starting with http:// or https://');
      return;
    }

    fetchFromBackend(url);
  }

  function uploadToBackend(file) {
    showLoading();

    const formData = new FormData();
    formData.append('file', file);

    const xhr = new XMLHttpRequest();
    xhr.open('POST', 'http://localhost:8000/api/process', true);

    xhr.onload = function() {
      hideLoading();

      if (xhr.status === 200) {
        try {
          const response = JSON.parse(xhr.responseText);
          if (response.success && response.chapters) {
            storeReadingData(response.chapters);
            window.location.href = 'reader.html';
          } else {
            showError(response.message || 'Failed to process file.');
          }
        } catch (e) {
          showError('Invalid response from server.');
        }
      } else {
        showError('Server error. Please ensure the backend is running.');
      }
    };

    xhr.onerror = function() {
      hideLoading();
      showError('Cannot connect to backend. Please ensure it is running.');
    };

    xhr.send(formData);
  }

  function fetchFromBackend(url) {
    showLoading();

    const xhr = new XMLHttpRequest();
    xhr.open('POST', 'http://localhost:8000/api/process', true);
    xhr.setRequestHeader('Content-Type', 'application/json');

    xhr.onload = function() {
      hideLoading();

      if (xhr.status === 200) {
        try {
          const response = JSON.parse(xhr.responseText);
          if (response.success && response.chapters) {
            storeReadingData(response.chapters);
            window.location.href = 'reader.html';
          } else {
            showError(response.message || 'Failed to fetch URL.');
          }
        } catch (e) {
          showError('Invalid response from server.');
        }
      } else {
        showError('Server error. Please ensure the backend is running.');
      }
    };

    xhr.onerror = function() {
      hideLoading();
      showError('Cannot connect to backend. Please ensure it is running.');
    };

    xhr.send(JSON.stringify({ url: url }));
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

  function storeReadingData(chapters) {
    try {
      localStorage.setItem('readingData', JSON.stringify(chapters));
    } catch (e) {
      // Fallback for browsers without localStorage
      window.readingData = chapters;
    }
  }
}

// ============================================================================
// READER PAGE CONTROLLER
// ============================================================================

function initReaderPage() {
  // Load reading data
  let chapters = null;
  try {
    const stored = localStorage.getItem('readingData');
    if (stored) {
      chapters = JSON.parse(stored);
    }
  } catch (e) {
    chapters = window.readingData || null;
  }

  if (!chapters || chapters.length === 0) {
    alert('No reading data found. Returning to home page.');
    window.location.href = 'index.html';
    return;
  }

  // Reader state
  const state = {
    chapters: chapters,
    currentChapterIndex: 0,
    allWords: [],
    currentWordIndex: 0,
    targetSpeed: 500,
    isPlaying: false,
    timer: null,
    startTime: null,
    totalElapsedTime: 0
  };

  // Prepare all words with chapter boundaries
  prepareWords();

  // DOM elements
  const readerContainer = document.getElementById('readerContainer');
  const wordDisplay = document.getElementById('wordDisplay');
  const speedDisplay = document.getElementById('speedDisplay');
  const progressDisplay = document.getElementById('progressDisplay');
  const chapterInfo = document.getElementById('chapterInfo');
  const chapterList = document.getElementById('chapterList');
  const chapterItems = document.getElementById('chapterItems');
  const playPauseButton = document.getElementById('playPauseButton');
  const increaseSpeed = document.getElementById('increaseSpeed');
  const decreaseSpeed = document.getElementById('decreaseSpeed');
  const restartButton = document.getElementById('restartButton');
  const backButton = document.getElementById('backButton');
  const forwardButton = document.getElementById('forwardButton');
  const closeButton = document.getElementById('closeButton');
  const chapterCompleteScreen = document.getElementById('chapterCompleteScreen');
  const readingCompleteScreen = document.getElementById('readingCompleteScreen');
  const nextChapterButton = document.getElementById('nextChapterButton');
  const chaptersButton = document.getElementById('chaptersButton');
  const homeFromChapter = document.getElementById('homeFromChapter');
  const restartFromComplete = document.getElementById('restartFromComplete');
  const homeFromComplete = document.getElementById('homeFromComplete');
  const chapterCompleteTitle = document.getElementById('chapterCompleteTitle');
  const completionMessage = document.getElementById('completionMessage');
  const totalWordsInline = document.getElementById('totalWordsInline');
  const timeTakenInline = document.getElementById('timeTakenInline');

  // Initialize UI
  updateSpeedDisplay();
  updateProgressDisplay();
  renderChapterList();
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
    const key = e.keyCode || e.which;

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
    for (let i = 0; i < state.chapters.length; i++) {
      const chapter = state.chapters[i];
      const words = tokenizeText(chapter.text);
      for (let j = 0; j < words.length; j++) {
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
    playPauseButton.innerHTML = '❚❚ Pause';

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
    playPauseButton.innerHTML = '▶ Play';
    updateChapterInfo();
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

    const wordData = state.allWords[state.currentWordIndex];
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
    const currentSpeed = getCurrentSpeed(state.currentWordIndex - 1, state.targetSpeed);
    const interval = getIntervalMs(currentSpeed);

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
    const current = state.currentWordIndex + 1;
    const total = state.allWords.length;
    const percentage = Math.round((current / total) * 100);
    progressDisplay.textContent = percentage + '%';
  }

  function updateChapterInfo() {
    if (state.chapters.length > 1) {
      const chapter = state.chapters[state.currentChapterIndex];
      chapterInfo.textContent = chapter.title;
      chapterInfo.className = 'chapter-info visible';
    }
  }

  function renderChapterList() {
    if (state.chapters.length <= 1) {
      return;
    }

    chapterItems.innerHTML = '';
    for (let i = 0; i < state.chapters.length; i++) {
      const chapter = state.chapters[i];
      const item = document.createElement('div');
      item.className = 'chapter-item';
      if (i === state.currentChapterIndex) {
        item.className = 'chapter-item current';
      }
      item.textContent = chapter.title;
      item.setAttribute('data-index', i);

      addEvent(item, 'click', function() {
        const index = parseInt(this.getAttribute('data-index'));
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
    for (let i = 0; i < state.allWords.length; i++) {
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
    const wasPlaying = state.isPlaying;
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
    const chapter = state.chapters[state.currentChapterIndex];
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

    const totalWords = state.allWords.length;
    const timeTaken = state.totalElapsedTime + (state.startTime ? Date.now() - state.startTime : 0);

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
    window.location.href = 'index.html';
  }
}

// ============================================================================
// THEME DETECTION - Pure JS Color Inversion (Old Browser Compatible)
// ============================================================================

function invertColor(hex) {
  // Convert hex to RGB
  hex = hex.replace('#', '');
  var r = parseInt(hex.substr(0, 2), 16);
  var g = parseInt(hex.substr(2, 2), 16);
  var b = parseInt(hex.substr(4, 2), 16);

  // Invert
  r = 255 - r;
  g = 255 - g;
  b = 255 - b;

  // Convert back to hex
  return '#' +
    ('0' + r.toString(16)).slice(-2) +
    ('0' + g.toString(16)).slice(-2) +
    ('0' + b.toString(16)).slice(-2);
}

function applyColorScheme(isDark) {
  var body = document.body;
  var html = document.documentElement;

  if (isDark) {
    // Dark theme colors (inverted)
    body.style.background = '#1a1a1a';
    body.style.color = '#e0e0e0';
    html.style.background = '#1a1a1a';
  } else {
    // Light theme colors (original)
    body.style.background = '#fff';
    body.style.color = '#000';
    html.style.background = '#fff';
  }

  // Apply to all elements that need color changes
  var elements = {
    '.mode-tab': { bg: isDark ? '#2a2a2a' : '#f0f0f0', color: isDark ? '#e0e0e0' : '#000', border: isDark ? '#444' : '#ddd' },
    '.mode-tab.active': { bg: isDark ? '#fff' : '#000', color: isDark ? '#000' : '#fff', border: isDark ? '#fff' : '#000' },
    '#pasteText, #urlInput': { bg: isDark ? '#2a2a2a' : '#fff', color: isDark ? '#e0e0e0' : '#000', border: isDark ? '#444' : '#ddd' },
    '.file-upload-area': { border: isDark ? '#444' : '#ddd' },
    '.file-button, .start-button, .speed-button, .control-button': { bg: isDark ? '#fff' : '#000', color: isDark ? '#000' : '#fff' },
    '#wordDisplay, .app-title': { color: isDark ? '#e0e0e0' : '#000' },
    '.reader-controls, .bottom-controls': { bg: isDark ? 'rgba(26, 26, 26, 0.95)' : 'rgba(255, 255, 255, 0.95)', border: isDark ? '#444' : '#ddd' },
    '.paused-indicator, .progress-display': { color: isDark ? '#999' : '#666' },
    '.chapter-list': { bg: isDark ? '#2a2a2a' : '#f9f9f9', border: isDark ? '#444' : '#ddd' },
    '.chapter-item': { bg: isDark ? '#333' : '#fff', color: isDark ? '#e0e0e0' : '#000', border: isDark ? '#444' : '#ddd' }
  };

  for (var selector in elements) {
    var els = document.querySelectorAll(selector);
    var colors = elements[selector];
    for (var i = 0; i < els.length; i++) {
      if (colors.bg) els[i].style.background = colors.bg;
      if (colors.color) els[i].style.color = colors.color;
      if (colors.border) els[i].style.borderColor = colors.border;
    }
  }
}

function detectAndApplyTheme() {
  // Check for dark mode preference
  var darkModeQuery = window.matchMedia && window.matchMedia('(prefers-color-scheme: dark)');

  if (darkModeQuery) {
    applyColorScheme(darkModeQuery.matches);

    // Listen for theme changes
    if (darkModeQuery.addEventListener) {
      darkModeQuery.addEventListener('change', function(e) {
        applyColorScheme(e.matches);
      });
    } else if (darkModeQuery.addListener) {
      // Fallback for older browsers
      darkModeQuery.addListener(function(e) {
        applyColorScheme(e.matches);
      });
    }
  }
}

// ============================================================================
// INITIALIZATION
// ============================================================================

addEvent(window, 'load', function() {
  // Apply theme first
  detectAndApplyTheme();

  // Detect which page we're on
  if (document.getElementById('startButton')) {
    initHomePage();
  } else if (document.getElementById('readerContainer')) {
    initReaderPage();
  }
});
