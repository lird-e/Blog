@echo off
chcp 65001 >nul
cd /d "%~dp0"
if not exist ".venv\Scripts\python.exe" (
  echo 未找到 .venv 虚拟环境，请先运行：
  echo   python -m venv .venv
  echo   .venv\Scripts\pip install -r requirements.txt
  pause
  exit /b 1
)
call .venv\Scripts\activate
rem public/ 由 build.py 生成且已被 .gitignore 忽略，首次运行前先构建一次
python build.py
python serve.py
pause
