function post(path, params, method) {
    method = method || "post"; // Set method to post by default if not specified.

    // The rest of this code assumes you are not using a library.
    // It can be made less wordy if you use one.
    var form = document.createElement("form");
    form.setAttribute("method", method);
    form.setAttribute("action", path);

    for(var key in params) {
        if(params.hasOwnProperty(key)) {
            var hiddenField = document.createElement("input");
            hiddenField.setAttribute("type", "hidden");
            hiddenField.setAttribute("name", key);
            hiddenField.setAttribute("value", params[key]);

            form.appendChild(hiddenField);
         }
    }

    document.body.appendChild(form);
    form.submit();
}

function serialize(obj) {
    var str = [];
    for(var p in obj) {
        if (obj.hasOwnProperty(p)) {
            str.push(encodeURIComponent(p) + "=" + encodeURIComponent(obj[p]));
        }
    }
    return str.join("&");
}

// Accessing nested JavaScript objects with string key
// http://stackoverflow.com/questions/6491463/accessing-nested-javascript-objects-with-string-key
function getPropertyByString(o, s) {
    s = s.replace(/\[(\w+)\]/g, '.$1'); // convert indexes to properties
    s = s.replace(/^\./, '');           // strip a leading dot
    var a = s.split('.');
    for (var i = 0, n = a.length; i < n; ++i) {
        var k = a[i];
        if (k in o) {
            o = o[k];
        } else {
            return;
        }
    }
    return o;
}

function createJSONEditor(jsonObject, annoString, jsoneditorNode, annoeditorcontainerNode) {
    var jsoneditor_container = jsoneditorNode;
    var jsoneditor_options = {
        modes: ["tree", "code"],
        mode: "code",
        onChange: json2anno,
        onError: function (err) {
            alert(err.toString());
        },
    };
    var annoeditor_container = annoeditorcontainerNode;
    var annoeditor_options = {
        mode: "code",
    };
    var jsoneditor = new JSONEditor(jsoneditor_container, jsoneditor_options, jsonObject);
    var annoeditors = {};
    var annotations = getPropertyByString(jsonObject, annoString);
    if (annotations) {
        Object.keys(annotations).forEach(function(key) {
            if (key.startsWith('config/')) {
                var header = document.createElement('h5');
                header.textContent = annoString + "." + key;
                var container = document.createElement('div');
                container.setAttribute('class', 'annoeditor');
                annoeditor_container.appendChild(header);
                annoeditor_container.appendChild(container);
                annoeditor_options.onChange = function() { anno2json(key); };
                var annoeditor = new JSONEditor(container, annoeditor_options);
                annoeditor.setText(annotations[key]);
                annoeditor.aceEditor.getSession().setMode('ace/mode/text');
                annoeditors[key] = annoeditor;
            }
        });
    }
    function anno2json(key) {
        Object.keys(annoeditors).forEach(function(editorKey) {
            if (key === editorKey) {
                var object = jsoneditor.get();
                var annotations = getPropertyByString(object, annoString);
                if (annotations) {
                    annotations[key] = annoeditors[key].getText();
                    jsoneditor.set(object);
                }
            }
        });
    }
    function json2anno() {
        var object = jsoneditor.get();
        var annotations = getPropertyByString(object, annoString);
        if (annotations) {
            Object.keys(annotations).forEach(function(key) {
                if (annoeditors[key]) {
                    annoeditors[key].setText(annotations[key]);
                }
            });
        }
    }
    return {
        editor: jsoneditor,
        onChange: json2anno,
    };
}
