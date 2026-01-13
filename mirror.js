const fs = require('fs');
const path = require('path');
const https = require('https');
const { URL } = require('url');

const BASE_URL = 'https://ofvergards.ax';
const OUTPUT_DIR = path.join(__dirname, 'website_mirror');

if (!fs.existsSync(OUTPUT_DIR)) {
    fs.mkdirSync(OUTPUT_DIR, { recursive: true });
}

const visited = new Set();
const queue = ['/'];

function downloadFile(url, filepath) {
    return new Promise((resolve, reject) => {
        const file = fs.createWriteStream(filepath);
        https.get(url, (response) => {
            if (response.statusCode !== 200) {
                // consume response data to free up memory
                response.resume();
                // reject(new Error(`Request Failed. Status Code: ${response.statusCode}`));
                 // Just ignore failures (like 404s) for now to keep going
                 console.log(`Failed to download ${url}: ${response.statusCode}`);
                 resolve();
                return;
            }
            response.pipe(file);
            file.on('finish', () => {
                file.close();
                console.log(`Downloaded: ${url} -> ${filepath}`);
                resolve();
            });
        }).on('error', (err) => {
            fs.unlink(filepath, () => {}); // Delete the file async. (But we don't check for this)
            // reject(err.message);
            console.error(`Error downloading ${url}: ${err.message}`);
             resolve();
        });
    });
}

function ensureDirectoryExistence(filePath) {
    const dirname = path.dirname(filePath);
    if (fs.existsSync(dirname)) {
        return true;
    }
    ensureDirectoryExistence(dirname);
    fs.mkdirSync(dirname);
}

function getPathFromUrl(urlPath) {
    // If it ends with /, append index.html
    if (urlPath.endsWith('/')) {
        return path.join(OUTPUT_DIR, urlPath, 'index.html');
    }
    // If it has no extension, assume it's a page and append /index.html ??
    // Or just append .html?
    // Let's check if it looks like a file
    if (!path.extname(urlPath)) {
         return path.join(OUTPUT_DIR, urlPath, 'index.html');
    }
    return path.join(OUTPUT_DIR, urlPath);
}

async function processPage(urlPath) {
    if (visited.has(urlPath)) return;
    visited.add(urlPath);

    console.log(`Processing: ${urlPath}`);
    const fullUrl = new URL(urlPath, BASE_URL).toString();

    // Fetch the page content
    const content = await new Promise((resolve, reject) => {
        https.get(fullUrl, (res) => {
            let data = '';
            res.on('data', (chunk) => data += chunk);
            res.on('end', () => resolve(data));
            res.on('error', reject);
        });
    });

    // Save the file
    const localPath = getPathFromUrl(urlPath);
    ensureDirectoryExistence(localPath);
    fs.writeFileSync(localPath, content);

    // Find links (href and src)
    // Very simple regex for href="..." and src="..."
    const linkRegex = /(?:href|src)=["']([^"']+)["']/g;
    let match;
    while ((match = linkRegex.exec(content)) !== null) {
        let link = match[1];

        // Optimize: ignore internal anchors, mailto, tel
        if (link.startsWith('#') || link.startsWith('mailto:') || link.startsWith('tel:')) continue;

        // Make relative links absolute with respect to the domain
        let absoluteUrl;
        try {
             absoluteUrl = new URL(link, fullUrl);
        } catch (e) {
            continue;
        }

        // Only process links on the same domain
        if (absoluteUrl.origin !== new URL(BASE_URL).origin) continue;

        const nextPath = absoluteUrl.pathname;

        // If it's a static asset (css, js, images, font), download it
        if (/\.(css|js|png|jpg|jpeg|gif|svg|woff|woff2|ttf|eot|ico)$/i.test(nextPath)) {
            const assetLocalPath = path.join(OUTPUT_DIR, nextPath);
            ensureDirectoryExistence(assetLocalPath);
            if (!fs.existsSync(assetLocalPath)) { // simple cache check
                 await downloadFile(absoluteUrl.toString(), assetLocalPath);
            }
        }
        // If it looks like a page and we haven't visited, add to queue
        else if (!visited.has(nextPath)) {
            // Check if it's likely a page
            // We can add it to the queue. BFS will handle it.
            queue.push(nextPath);
        }
    }
}

async function main() {
    while (queue.length > 0) {
        const nextUrl = queue.shift();
        try {
            await processPage(nextUrl);
        } catch (e) {
            console.error(`Failed to process ${nextUrl}:`, e);
        }
    }
    console.log('Mirroring complete.');
}

main();
