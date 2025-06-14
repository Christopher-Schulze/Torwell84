#!/bin/sh
set -e
(cd backend && make clean all)
(cd ui && npm install && npm run build)
