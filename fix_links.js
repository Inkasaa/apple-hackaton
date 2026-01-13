const fs = require('fs');
const path = require('path');

const MIRROR_DIR = path.join(__dirname, 'website_mirror');
const BASE_URL = 'https://ofvergards.ax';

function getAllFiles(dirPath, arrayOfFiles) {
    const files = fs.readdirSync(dirPath);
    arrayOfFiles = arrayOfFiles || [];

    files.forEach(function (file) {
        if (fs.statSync(dirPath + "/" + file).isDirectory()) {
            arrayOfFiles = getAllFiles(dirPath + "/" + file, arrayOfFiles);
        } else {
            arrayOfFiles.push(path.join(dirPath, "/", file));
        }
    });

    return arrayOfFiles;
}

function processFile(filePath) {
    const ext = path.extname(filePath).toLowerCase();
    if (ext !== '.html' && ext !== '.css') return;

    let content = fs.readFileSync(filePath, 'utf8');
    let originalContent = content;

    // Regex to capture URLs starting with base url
    // We want to match https://ofvergards.ax/some/path
    // inside specific contexts like href="", src="", url(), strings?
    // Safe approach: replace all occurrences of the base url?
    // Might be dangerous if it appears in text.
    // Better: (href|src|url|srcset)=["'](https://ofvergards.ax[^"']+)["']
    // Also handle unquoted url(...) in css.

    // HTML attributes
    // Capture group 1: attribute name (href, src, etc)
    // Capture group 2: quote
    // Capture group 3: The URL
    // Capture group 4: quote
    const htmlRegex = /(href|src|srcset|action)=([\"'])(https:\/\/ofvergards\.ax[^\"'\s]*)\2/gi;

    content = content.replace(htmlRegex, (match, attr, quote, url) => {
        const newUrl = convertToRelative(url, filePath);
        return `${attr}=${quote}${newUrl}${quote}`;
    });

    // CSS url()
    // url(https://...) or url("https://...")
    const cssRegex = /url\(([\"']?)(https:\/\/ofvergards\.ax[^\"'\)]+)\1\)/gi;
    content = content.replace(cssRegex, (match, quote, url) => {
        const newUrl = convertToRelative(url, filePath);
        return `url(${quote}${newUrl}${quote})`;
    });

    // JSON files? Skipping for now.

    if (content !== originalContent) {
        fs.writeFileSync(filePath, content);
        console.log(`Updated ${filePath}`);
    }
}

function convertToRelative(absoluteUrl, currentFilePath) {
    // 1. Determine the logical path of the target
    let targetUrl;
    try {
        targetUrl = new URL(absoluteUrl);
    } catch (e) {
        return absoluteUrl; // fallback
    }

    if (targetUrl.origin !== new URL(BASE_URL).origin) {
        return absoluteUrl;
    }

    let urlPath = targetUrl.pathname;

    // Normalize path to file system path
    // If it ends in /, index.html
    // If no extension, is it a directory? Assume yes if no "." in last segment? 
    // This is tricky. In mirror.js we said:
    // if (urlPath.endsWith('/')) -> index.html
    // else if (!path.extname(urlPath)) -> index.html
    // else -> as is

    let targetFsPath;
    if (urlPath.endsWith('/')) {
        targetFsPath = path.join(urlPath, 'index.html');
    } else if (path.extname(urlPath) === '') {
        targetFsPath = path.join(urlPath, 'index.html');
        // Note: This matches mirror.js logic. 
        // BUT if the file downloaded was actually an image without extension (rare), this breaks.
        // Assuming standard web pages.
    } else {
        targetFsPath = urlPath;
    }

    // Now calculate relative path from currentFilePath to targetFsPath
    // currentFilePath is absolute local path.
    // targetFsPath is absolute 'web' path (starting with /) relative to MIRROR_DIR

    const absTargetFsPath = path.join(MIRROR_DIR, targetFsPath);
    const absCurrentDir = path.dirname(currentFilePath);

    let relativePath = path.relative(absCurrentDir, absTargetFsPath);

    // If it is the file itself, maybe just #? No, user wants links. 
    // path.relative returns "" if same.
    if (relativePath === '') relativePath = './' + path.basename(absTargetFsPath);

    // Keep query params and hash
    if (targetUrl.search) relativePath += targetUrl.search;
    if (targetUrl.hash) relativePath += targetUrl.hash;

    return relativePath;
}

const files = getAllFiles(MIRROR_DIR);
files.forEach(processFile);
console.log('Link fixing complete.');
