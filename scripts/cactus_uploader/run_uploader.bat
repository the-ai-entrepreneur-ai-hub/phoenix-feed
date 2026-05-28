@echo off
set PHOENIX_FEED_URL=https://feed.cactuswatch.com
"C:\cactus\venv\Scripts\python.exe" "C:\cactus\cactus_uploader.py" >> "C:\cactus\logs\uploader.stdout" 2>> "C:\cactus\logs\uploader.stderr"
