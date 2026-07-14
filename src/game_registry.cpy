       01 REGISTRY-GAME-COUNT PIC 9(2) VALUE 3.
       01 REGISTRY-DEFAULT-MINIMUM-LAUNCHABLE PIC 9(2) VALUE 1.
       01 REGISTRY-DEFAULT-STATUS-CACHE-TTL PIC X(16) VALUE "5s".

       01 GAME-REGISTRY-STORAGE.
          05 FILLER.
             10 FILLER PIC X(16) VALUE "trump".
             10 FILLER PIC X(64) VALUE "Trump vs. Shakespeare".
             10 FILLER PIC X(16) VALUE "trump".
             10 FILLER PIC X(64) VALUE "/play/trump".
             10 FILLER PIC X(32) VALUE "TRUMP_UPSTREAM".
             10 FILLER PIC X(128) VALUE "http://trump-vs-shakespeare:8000".
             10 FILLER PIC X(32) VALUE "/readyz".
             10 FILLER PIC X(32) VALUE "TRUMP_ENABLED".
             10 FILLER PIC X(32) VALUE "TRUMP_REQUIRED".
             10 FILLER PIC X(32) VALUE "TRUMP_MAX_LATENCY_MS".
             10 FILLER PIC X VALUE "Y".
             10 FILLER PIC X VALUE "Y".
             10 FILLER PIC 9(6) VALUE 002500.
             10 FILLER PIC X(96) VALUE "TRUMPSCRIPT / SPL / ASSEMBLY / PYTHON".
             10 FILLER PIC X(256) VALUE "A simultaneous-turn debate duel with local and online play, server-authoritative combat, and live WebSocket rooms.".
             10 FILLER PIC X(96) VALUE "1v1 local and online".
             10 FILLER PIC X(96) VALUE "Real Assembly combat runtime".
             10 FILLER PIC X(96) VALUE "Ephemeral room system".
             10 FILLER PIC X(48) VALUE "Start duel".
             10 FILLER PIC X(160) VALUE "https://github.com/Pepitodrop/TrumpVsShakespeare".
          05 FILLER.
             10 FILLER PIC X(16) VALUE "golf".
             10 FILLER PIC X(64) VALUE "Crazy Mini Golf".
             10 FILLER PIC X(16) VALUE "golf".
             10 FILLER PIC X(64) VALUE "/play/golf".
             10 FILLER PIC X(32) VALUE "GOLF_UPSTREAM".
             10 FILLER PIC X(128) VALUE "http://crazy-mini-golf:8080".
             10 FILLER PIC X(32) VALUE "/healthz".
             10 FILLER PIC X(32) VALUE "GOLF_ENABLED".
             10 FILLER PIC X(32) VALUE "GOLF_REQUIRED".
             10 FILLER PIC X(32) VALUE "GOLF_MAX_LATENCY_MS".
             10 FILLER PIC X VALUE "Y".
             10 FILLER PIC X VALUE "Y".
             10 FILLER PIC 9(6) VALUE 002500.
             10 FILLER PIC X(96) VALUE "BRAINFUCK / TYPESCRIPT / R".
             10 FILLER PIC X(256) VALUE "A complete nine-hole browser minigolf game whose authoritative state transitions run through a protected Brainfuck engine.".
             10 FILLER PIC X(96) VALUE "Nine designed holes".
             10 FILLER PIC X(96) VALUE "Mouse, touch, and keyboard".
             10 FILLER PIC X(96) VALUE "Local score persistence".
             10 FILLER PIC X(48) VALUE "Tee off".
             10 FILLER PIC X(160) VALUE "https://github.com/Pepitodrop/CrazyMiniGolf".
          05 FILLER.
             10 FILLER PIC X(16) VALUE "race".
             10 FILLER PIC X(64) VALUE "Crazy Race".
             10 FILLER PIC X(16) VALUE "race".
             10 FILLER PIC X(64) VALUE "/play/race".
             10 FILLER PIC X(32) VALUE "RACE_UPSTREAM".
             10 FILLER PIC X(128) VALUE "http://crazy-race:8080".
             10 FILLER PIC X(32) VALUE "/health".
             10 FILLER PIC X(32) VALUE "RACE_ENABLED".
             10 FILLER PIC X(32) VALUE "RACE_REQUIRED".
             10 FILLER PIC X(32) VALUE "RACE_MAX_LATENCY_MS".
             10 FILLER PIC X VALUE "Y".
             10 FILLER PIC X VALUE "Y".
             10 FILLER PIC 9(6) VALUE 002500.
             10 FILLER PIC X(96) VALUE "RUST / R / TRUMPSCRIPT / PIET".
             10 FILLER PIC X(256) VALUE "A server-rendered 1v1 racing game with an R-generated circuit, TrumpScript commentary, and a Piet-powered boost oracle.".
             10 FILLER PIC X(96) VALUE "Online and pass-and-play".
             10 FILLER PIC X(96) VALUE "Fresh generated circuits".
             10 FILLER PIC X(96) VALUE "Adaptive Piet boosts".
             10 FILLER PIC X(48) VALUE "Start race".
             10 FILLER PIC X(160) VALUE "https://github.com/Pepitodrop/CrazyRaceGame".

       01 GAME-REGISTRY REDEFINES GAME-REGISTRY-STORAGE.
          05 REGISTRY-GAME OCCURS 3 TIMES.
             10 REG-GAME-KEY PIC X(16).
             10 REG-GAME-NAME PIC X(64).
             10 REG-GAME-CSS-CLASS PIC X(16).
             10 REG-GAME-PREFIX PIC X(64).
             10 REG-GAME-UPSTREAM-ENV PIC X(32).
             10 REG-GAME-DEFAULT-UPSTREAM PIC X(128).
             10 REG-GAME-HEALTH-PATH PIC X(32).
             10 REG-GAME-ENABLED-ENV PIC X(32).
             10 REG-GAME-REQUIRED-ENV PIC X(32).
             10 REG-GAME-LATENCY-ENV PIC X(32).
             10 REG-GAME-DEFAULT-ENABLED PIC X.
             10 REG-GAME-DEFAULT-REQUIRED PIC X.
             10 REG-GAME-DEFAULT-MAX-LATENCY PIC 9(6).
             10 REG-GAME-STACK PIC X(96).
             10 REG-GAME-DESCRIPTION PIC X(256).
             10 REG-GAME-FEATURE-1 PIC X(96).
             10 REG-GAME-FEATURE-2 PIC X(96).
             10 REG-GAME-FEATURE-3 PIC X(96).
             10 REG-GAME-ACTION PIC X(48).
             10 REG-GAME-REPOSITORY PIC X(160).
