@echo off
:: Octopus Startup Script for Windows
:: This batch file runs the PowerShell startup script

PowerShell -ExecutionPolicy Bypass -File "%~dp0start.ps1"
