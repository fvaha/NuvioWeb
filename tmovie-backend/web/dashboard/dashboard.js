(function () {
    'use strict';

    function $(id) {
        return document.getElementById(id);
    }

    function httpGet(url, done, fail) {
        var xhr = new XMLHttpRequest();
        xhr.open('GET', url, true);
        xhr.onreadystatechange = function () {
            if (xhr.readyState !== 4) {
                return;
            }
            if (xhr.status >= 200 && xhr.status < 300) {
                try {
                    done(JSON.parse(xhr.responseText));
                } catch (e) {
                    fail(e);
                }
            } else {
                fail(new Error('HTTP ' + xhr.status));
            }
        };
        xhr.onerror = function () {
            fail(new Error('network'));
        };
        xhr.send(null);
    }

    function badge(label, ok) {
        var li = document.createElement('li');
        li.textContent = label + ': ' + (ok ? 'yes' : 'no');
        li.className = ok ? 'ok' : 'no';
        return li;
    }

    function loadStatus() {
        httpGet('/api/v1/status', function (data) {
            var ul = $('integrations');
            ul.innerHTML = '';
            var integ = data.integrations || {};
            var keys = Object.keys(integ).sort();
            for (var i = 0; i < keys.length; i++) {
                ul.appendChild(badge(keys[i], !!integ[keys[i]]));
            }
            var p = data.paths || {};
            $('paths').textContent =
                'SQLite: ' + (p.sqlite || '') +
                ' | public: ' + (p.public_dir || '') +
                ' | dotenv: ' + (p.dotenv_hint || '') +
                ' | Prevodi backend: ' + (p.prevodi_backend_hint || '');
            $('endpoints').textContent = JSON.stringify(data.endpoints || {}, null, 2);
        }, function () {
            $('integrations').innerHTML = '';
            $('integrations').appendChild(badge('status fetch', false));
        });
    }

    $('search-form').addEventListener('submit', function (ev) {
        ev.preventDefault();
        var q = ($('q').value || '').replace(/^\s+|\s+$/g, '');
        if (!q) {
            return;
        }
        var url = '/api/v1/search?query=' + encodeURIComponent(q);
        httpGet(url, function (data) {
            $('search-out').textContent = JSON.stringify(data, null, 2);
        }, function (err) {
            $('search-out').textContent = String(err);
        });
    });

    loadStatus();
})();
