self.addEventListener('install', event => {
  event.waitUntil(
    caches.open('torwell-cache').then(cache => cache.add('/'))
  );
});

self.addEventListener('fetch', event => {
  event.respondWith(
    caches.match(event.request).then(r => r || fetch(event.request))
  );
});
