# Няшный котик 3000 — Proxy Client 1.1.0 fix8

Открытый тестовый кандидат расширения Chrome/Chromium и локального Native
Messaging Helper. Release `v1.1.0-fix8` предназначен для ручной проверки и не
является стабильной массовой версией.

Постоянный ID расширения: `fgogfmjjadljkpiejmlmhoaafepfnooo`.

## Установка расширения

1. Скачайте `NyashnyKotik-Extension-1.1.0-fix8.zip` из Release и распакуйте.
2. Откройте `chrome://extensions`, включите режим разработчика и выберите
   «Загрузить распакованное расширение».
3. Откройте Котика и скопируйте показанную им команду установки Helper.

`manifest.key` сохранён внутри `manifest.json`, поэтому обновление поверх этой
версии сохраняет Extension ID и `chrome.storage.local`.

## macOS Apple Silicon

```sh
curl -fsSL https://raw.githubusercontent.com/sismeliostroy/Nyashny-Kotik-Extension/main/install/macos.sh | bash -s -- fgogfmjjadljkpiejmlmhoaafepfnooo
```

Установка выполняется только для текущего пользователя, без `sudo`, `/Library`,
LaunchDaemon, TUN, системного proxy и изменения Gatekeeper. Intel Mac в этом
кандидате не поддерживается и отклоняется до изменения файлов.

Удаление:

```sh
curl -fsSL https://raw.githubusercontent.com/sismeliostroy/Nyashny-Kotik-Extension/main/uninstall/macos.sh | bash
```

## Windows x64

В обычном Windows PowerShell 5.1:

```powershell
$env:KOTIK_EXT_ID='fgogfmjjadljkpiejmlmhoaafepfnooo'; iwr -useb https://raw.githubusercontent.com/sismeliostroy/Nyashny-Kotik-Extension/main/install/windows.ps1 | iex
```

Используются только `%LOCALAPPDATA%` и HKCU; UAC, служба, драйвер и TUN не нужны.

Удаление:

```powershell
iwr -useb https://raw.githubusercontent.com/sismeliostroy/Nyashny-Kotik-Extension/main/uninstall/windows.ps1 | iex
```

## Безопасность установки

Установщики привязаны к тегу `v1.1.0-fix8`. Каждый gzip-ресурс проверяется по
собственному `.sha256` до распаковки и запуска. Замена каталога транзакционная:
при ошибке предыдущая рабочая версия возвращается. Все локальные listeners
ядер создаются Helper только на `127.0.0.1`.

Архитектура и модель безопасности описаны в `docs/ARCHITECTURE.md` и
`docs/SECURITY.md`. Лицензии — в `LICENSE`, `THIRD_PARTY_NOTICES.md` и
`LICENSES/`.
