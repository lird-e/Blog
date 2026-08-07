@echo off
chcp 65001 >nul
cd /d "%~dp0"
if not exist ".venv\Scripts\python.exe" (
  echo 未找到 .venv 虚拟环境，请先运行：
  echo   python -m venv .venv
  echo   .venv\Scripts\pip install markdown pygments
  pause
  exit /b 1
)
call .venv\Scripts\activate
python serve.py
pause
