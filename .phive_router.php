<?php
        $path = parse_url($_SERVER["REQUEST_URI"], PHP_URL_PATH);
        $file = __DIR__ . $path;

        if (is_dir($file)) {
            $file = rtrim($file, "/") . "/index.php";
        }

        if (file_exists($file) && pathinfo($file, PATHINFO_EXTENSION) === "php") {
            ob_start();
            include $file;
            $content = ob_get_clean();
            if (strpos($content, '</body>') !== false) {
                echo str_replace("</body>", "        <script>            (function() {                const socket = new WebSocket('ws://192.168.1.13:9001');                socket.onmessage = (msg) => {                     if (msg.data === 'reload') {                        console.log('Phive: Reloading...');                        window.location.reload();                     }                };                socket.onopen = () => console.log('Phive: Live Reload Connected');                socket.onerror = () => console.error('Phive: Live Reload Connection Error');            })();        </script></body>", $content);
            } else {
                echo $content . "        <script>            (function() {                const socket = new WebSocket('ws://192.168.1.13:9001');                socket.onmessage = (msg) => {                     if (msg.data === 'reload') {                        console.log('Phive: Reloading...');                        window.location.reload();                     }                };                socket.onopen = () => console.log('Phive: Live Reload Connected');                socket.onerror = () => console.error('Phive: Live Reload Connection Error');            })();        </script>";
            }
        } else {
            return false;
        }
        