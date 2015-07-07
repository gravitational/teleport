/*
* jQuery File Download Plugin v1.4.4
*
* http://www.johnculviner.com
*
* Copyright (c) 2013 - John Culviner
*
* Licensed under the MIT license:
*   http://www.opensource.org/licenses/mit-license.php
*
* !!!!NOTE!!!!
* You must also write a cookie in conjunction with using this plugin as mentioned in the orignal post:
* http://johnculviner.com/jquery-file-download-plugin-for-ajax-like-feature-rich-file-downloads/
* !!!!NOTE!!!!
*/

(function($, window){
	// i'll just put them here to get evaluated on script load
	var htmlSpecialCharsRegEx = /[<>&\r\n"']/gm;
	var htmlSpecialCharsPlaceHolders = {
				'<': 'lt;',
				'>': 'gt;',
				'&': 'amp;',
				'\r': "#13;",
				'\n': "#10;",
				'"': 'quot;',
				"'": '#39;' /*single quotes just to be safe, IE8 doesn't support &apos;, so use &#39; instead */
	};

$.extend({
    //
    //$.fileDownload('/path/to/url/', options)
    //  see directly below for possible 'options'
    fileDownload: function (fileUrl, options) {

        //provide some reasonable defaults to any unspecified options below
        var settings = $.extend({

            //
            //Requires jQuery UI: provide a message to display to the user when the file download is being prepared before the browser's dialog appears
            //
            preparingMessageHtml: null,

            //
            //Requires jQuery UI: provide a message to display to the user when a file download fails
            //
            failMessageHtml: null,

            //
            //the stock android browser straight up doesn't support file downloads initiated by a non GET: http://code.google.com/p/android/issues/detail?id=1780
            //specify a message here to display if a user tries with an android browser
            //if jQuery UI is installed this will be a dialog, otherwise it will be an alert
            //Set to null to disable the message and attempt to download anyway
            //
            androidPostUnsupportedMessageHtml: "Unfortunately your Android browser doesn't support this type of file download. Please try again with a different browser.",

            //
            //Requires jQuery UI: options to pass into jQuery UI Dialog
            //
            dialogOptions: { modal: true },

            //
            //a function to call while the dowload is being prepared before the browser's dialog appears
            //Args:
            //  url - the original url attempted
            //
            prepareCallback: function (url) { },

            //
            //a function to call after a file download dialog/ribbon has appeared
            //Args:
            //  url - the original url attempted
            //
            successCallback: function (url) { },

            //
            //a function to call after a file download dialog/ribbon has appeared
            //Args:
            //  responseHtml    - the html that came back in response to the file download. this won't necessarily come back depending on the browser.
            //                      in less than IE9 a cross domain error occurs because 500+ errors cause a cross domain issue due to IE subbing out the
            //                      server's error message with a "helpful" IE built in message
            //  url             - the original url attempted
            //  error           - original error cautch from exception
            //
            failCallback: function (responseHtml, url, error) { },

            //
            // the HTTP method to use. Defaults to "GET".
            //
            httpMethod: "GET",

            //
            // if specified will perform a "httpMethod" request to the specified 'fileUrl' using the specified data.
            // data must be an object (which will be $.param serialized) or already a key=value param string
            //
            data: null,

            //
            //a period in milliseconds to poll to determine if a successful file download has occured or not
            //
            checkInterval: 100,

            //
            //the cookie name to indicate if a file download has occured
            //
            cookieName: "fileDownload",

            //
            //the cookie value for the above name to indicate that a file download has occured
            //
            cookieValue: "true",

            //
            //the cookie path for above name value pair
            //
            cookiePath: "/",

            //
            //if specified it will be used when attempting to clear the above name value pair
            //useful for when downloads are being served on a subdomain (e.g. downloads.example.com)
            //
            cookieDomain: null,

            //
            //the title for the popup second window as a download is processing in the case of a mobile browser
            //
            popupWindowTitle: "Initiating file download...",

            //
            //Functionality to encode HTML entities for a POST, need this if data is an object with properties whose values contains strings with quotation marks.
            //HTML entity encoding is done by replacing all &,<,>,',",\r,\n characters.
            //Note that some browsers will POST the string htmlentity-encoded whilst others will decode it before POSTing.
            //It is recommended that on the server, htmlentity decoding is done irrespective.
            //
            encodeHTMLEntities: true

        }, options);

        var deferred = new $.Deferred();

        //Setup mobile browser detection: Partial credit: http://detectmobilebrowser.com/
        var userAgent = (navigator.userAgent || navigator.vendor || window.opera).toLowerCase();

        var isIos;                  //has full support of features in iOS 4.0+, uses a new window to accomplish this.
        var isAndroid;              //has full support of GET features in 4.0+ by using a new window. Non-GET is completely unsupported by the browser. See above for specifying a message.
        var isOtherMobileBrowser;   //there is no way to reliably guess here so all other mobile devices will GET and POST to the current window.

        if (/ip(ad|hone|od)/.test(userAgent)) {

            isIos = true;

        } else if (userAgent.indexOf('android') !== -1) {

            isAndroid = true;

        } else {

            isOtherMobileBrowser = /avantgo|bada\/|blackberry|blazer|compal|elaine|fennec|hiptop|playbook|silk|iemobile|iris|kindle|lge |maemo|midp|mmp|netfront|opera m(ob|in)i|palm( os)?|phone|p(ixi|re)\/|plucker|pocket|psp|symbian|treo|up\.(browser|link)|vodafone|wap|windows (ce|phone)|xda|xiino/i.test(userAgent) || /1207|6310|6590|3gso|4thp|50[1-6]i|770s|802s|a wa|abac|ac(er|oo|s\-)|ai(ko|rn)|al(av|ca|co)|amoi|an(ex|ny|yw)|aptu|ar(ch|go)|as(te|us)|attw|au(di|\-m|r |s )|avan|be(ck|ll|nq)|bi(lb|rd)|bl(ac|az)|br(e|v)w|bumb|bw\-(n|u)|c55\/|capi|ccwa|cdm\-|cell|chtm|cldc|cmd\-|co(mp|nd)|craw|da(it|ll|ng)|dbte|dc\-s|devi|dica|dmob|do(c|p)o|ds(12|\-d)|el(49|ai)|em(l2|ul)|er(ic|k0)|esl8|ez([4-7]0|os|wa|ze)|fetc|fly(\-|_)|g1 u|g560|gene|gf\-5|g\-mo|go(\.w|od)|gr(ad|un)|haie|hcit|hd\-(m|p|t)|hei\-|hi(pt|ta)|hp( i|ip)|hs\-c|ht(c(\-| |_|a|g|p|s|t)|tp)|hu(aw|tc)|i\-(20|go|ma)|i230|iac( |\-|\/)|ibro|idea|ig01|ikom|im1k|inno|ipaq|iris|ja(t|v)a|jbro|jemu|jigs|kddi|keji|kgt( |\/)|klon|kpt |kwc\-|kyo(c|k)|le(no|xi)|lg( g|\/(k|l|u)|50|54|e\-|e\/|\-[a-w])|libw|lynx|m1\-w|m3ga|m50\/|ma(te|ui|xo)|mc(01|21|ca)|m\-cr|me(di|rc|ri)|mi(o8|oa|ts)|mmef|mo(01|02|bi|de|do|t(\-| |o|v)|zz)|mt(50|p1|v )|mwbp|mywa|n10[0-2]|n20[2-3]|n30(0|2)|n50(0|2|5)|n7(0(0|1)|10)|ne((c|m)\-|on|tf|wf|wg|wt)|nok(6|i)|nzph|o2im|op(ti|wv)|oran|owg1|p800|pan(a|d|t)|pdxg|pg(13|\-([1-8]|c))|phil|pire|pl(ay|uc)|pn\-2|po(ck|rt|se)|prox|psio|pt\-g|qa\-a|qc(07|12|21|32|60|\-[2-7]|i\-)|qtek|r380|r600|raks|rim9|ro(ve|zo)|s55\/|sa(ge|ma|mm|ms|ny|va)|sc(01|h\-|oo|p\-)|sdk\/|se(c(\-|0|1)|47|mc|nd|ri)|sgh\-|shar|sie(\-|m)|sk\-0|sl(45|id)|sm(al|ar|b3|it|t5)|so(ft|ny)|sp(01|h\-|v\-|v )|sy(01|mb)|t2(18|50)|t6(00|10|18)|ta(gt|lk)|tcl\-|tdg\-|tel(i|m)|tim\-|t\-mo|to(pl|sh)|ts(70|m\-|m3|m5)|tx\-9|up(\.b|g1|si)|utst|v400|v750|veri|vi(rg|te)|vk(40|5[0-3]|\-v)|vm40|voda|vulc|vx(52|53|60|61|70|80|81|83|85|98)|w3c(\-| )|webc|whit|wi(g |nc|nw)|wmlb|wonu|x700|xda(\-|2|g)|yas\-|your|zeto|zte\-/i.test(userAgent.substr(0, 4));

        }

        var httpMethodUpper = settings.httpMethod.toUpperCase();

        if (isAndroid && httpMethodUpper !== "GET" && settings.androidPostUnsupportedMessageHtml) {
            //the stock android browser straight up doesn't support file downloads initiated by non GET requests: http://code.google.com/p/android/issues/detail?id=1780

            if ($().dialog) {
                $("<div>").html(settings.androidPostUnsupportedMessageHtml).dialog(settings.dialogOptions);
            } else {
                alert(settings.androidPostUnsupportedMessageHtml);
            }

            return deferred.reject();
        }

        var $preparingDialog = null;

        var internalCallbacks = {

            onPrepare: function (url) {

                //wire up a jquery dialog to display the preparing message if specified
                if (settings.preparingMessageHtml) {

                    $preparingDialog = $("<div>").html(settings.preparingMessageHtml).dialog(settings.dialogOptions);

                } else if (settings.prepareCallback) {

                    settings.prepareCallback(url);

                }

            },

            onSuccess: function (url) {

                //remove the perparing message if it was specified
                if ($preparingDialog) {
                    $preparingDialog.dialog('close');
                }

                settings.successCallback(url);

                deferred.resolve(url);
            },

            onFail: function (responseHtml, url, error) {

                //remove the perparing message if it was specified
                if ($preparingDialog) {
                    $preparingDialog.dialog('close');
                }

                //wire up a jquery dialog to display the fail message if specified
                if (settings.failMessageHtml) {
                    $("<div>").html(settings.failMessageHtml).dialog(settings.dialogOptions);
                }

                settings.failCallback(responseHtml, url, error);

                deferred.reject(responseHtml, url);
            }
        };

        internalCallbacks.onPrepare(fileUrl);

        //make settings.data a param string if it exists and isn't already
        if (settings.data !== null && typeof settings.data !== "string") {
            settings.data = $.param(settings.data);
        }


        var $iframe,
            downloadWindow,
            formDoc,
            $form;

        if (httpMethodUpper === "GET") {

            if (settings.data !== null) {
                //need to merge any fileUrl params with the data object

                var qsStart = fileUrl.indexOf('?');

                if (qsStart !== -1) {
                    //we have a querystring in the url

                    if (fileUrl.substring(fileUrl.length - 1) !== "&") {
                        fileUrl = fileUrl + "&";
                    }
                } else {

                    fileUrl = fileUrl + "?";
                }

                fileUrl = fileUrl + settings.data;
            }

            if (isIos || isAndroid) {

                downloadWindow = window.open(fileUrl);
                downloadWindow.document.title = settings.popupWindowTitle;
                window.focus();

            } else if (isOtherMobileBrowser) {

                window.location(fileUrl);

            } else {

                //create a temporary iframe that is used to request the fileUrl as a GET request
                $iframe = $("<iframe>")
                    .hide()
                    .prop("src", fileUrl)
                    .appendTo("body");
            }

        } else {

            var formInnerHtml = "";

            if (settings.data !== null) {

                $.each(settings.data.replace(/\+/g, ' ').split("&"), function () {

                    var kvp = this.split("=");

                    //Issue: When value contains sign '=' then the kvp array does have more than 2 items. We have to join value back
                    var k = kvp[0];
                    kvp.shift();
                    var v = kvp.join("=");
                    kvp = [k, v];

                    var key = settings.encodeHTMLEntities ? htmlSpecialCharsEntityEncode(decodeURIComponent(kvp[0])) : decodeURIComponent(kvp[0]);
                    if (key) {
                        var value = settings.encodeHTMLEntities ? htmlSpecialCharsEntityEncode(decodeURIComponent(kvp[1])) : decodeURIComponent(kvp[1]);
                    formInnerHtml += '<input type="hidden" name="' + key + '" value="' + value + '" />';
                    }
                });
            }

            if (isOtherMobileBrowser) {

                $form = $("<form>").appendTo("body");
                $form.hide()
                    .prop('method', settings.httpMethod)
                    .prop('action', fileUrl)
                    .html(formInnerHtml);

            } else {

                if (isIos) {

                    downloadWindow = window.open("about:blank");
                    downloadWindow.document.title = settings.popupWindowTitle;
                    formDoc = downloadWindow.document;
                    window.focus();

                } else {

                    $iframe = $("<iframe style='display: none' src='about:blank'></iframe>").appendTo("body");
                    formDoc = getiframeDocument($iframe);
                }

                formDoc.write("<html><head></head><body><form method='" + settings.httpMethod + "' action='" + fileUrl + "'>" + formInnerHtml + "</form>" + settings.popupWindowTitle + "</body></html>");
                $form = $(formDoc).find('form');
            }

            $form.submit();
        }


        //check if the file download has completed every checkInterval ms
        setTimeout(checkFileDownloadComplete, settings.checkInterval);


        function checkFileDownloadComplete() {
            //has the cookie been written due to a file download occuring?

            var cookieValue = settings.cookieValue;
            if(typeof cookieValue == 'string') {
                cookieValue = cookieValue.toLowerCase();
            }

            var lowerCaseCookie = settings.cookieName.toLowerCase() + "=" + cookieValue;

            if (document.cookie.toLowerCase().indexOf(lowerCaseCookie) > -1) {

                //execute specified callback
                internalCallbacks.onSuccess(fileUrl);

                //remove cookie
                var cookieData = settings.cookieName + "=; path=" + settings.cookiePath + "; expires=" + new Date(0).toUTCString() + ";";
                if (settings.cookieDomain) cookieData += " domain=" + settings.cookieDomain + ";";
                document.cookie = cookieData;

                //remove iframe
                cleanUp(false);

                return;
            }

            //has an error occured?
            //if neither containers exist below then the file download is occuring on the current window
            if (downloadWindow || $iframe) {

                //has an error occured?
                try {

                    var formDoc = downloadWindow ? downloadWindow.document : getiframeDocument($iframe);

                    if (formDoc && formDoc.body !== null && formDoc.body.innerHTML.length) {

                        var isFailure = true;

                        if ($form && $form.length) {
                            var $contents = $(formDoc.body).contents().first();

                            try {
                                if ($contents.length && $contents[0] === $form[0]) {
                                    isFailure = false;
                                }
                            } catch (e) {
                                if (e && e.number == -2146828218) {
                                    // IE 8-10 throw a permission denied after the form reloads on the "$contents[0] === $form[0]" comparison
                                    isFailure = true;
                                } else {
                                    throw e;
                                }
                            }
                        }

                        if (isFailure) {
                            // IE 8-10 don't always have the full content available right away, they need a litle bit to finish
                            setTimeout(function () {
                                internalCallbacks.onFail(formDoc.body.innerHTML, fileUrl);
                                cleanUp(true);
                            }, 100);

                            return;
                        }
                    }
                }
                catch (err) {

                    //500 error less than IE9
                    internalCallbacks.onFail('', fileUrl, err);

                    cleanUp(true);

                    return;
                }
            }


            //keep checking...
            setTimeout(checkFileDownloadComplete, settings.checkInterval);
        }

        //gets an iframes document in a cross browser compatible manner
        function getiframeDocument($iframe) {
            var iframeDoc = $iframe[0].contentWindow || $iframe[0].contentDocument;
            if (iframeDoc.document) {
                iframeDoc = iframeDoc.document;
            }
            return iframeDoc;
        }

        function cleanUp(isFailure) {

            setTimeout(function() {

                if (downloadWindow) {

                    if (isAndroid) {
                        downloadWindow.close();
                    }

                    if (isIos) {
                        if (downloadWindow.focus) {
                            downloadWindow.focus(); //ios safari bug doesn't allow a window to be closed unless it is focused
                            if (isFailure) {
                                downloadWindow.close();
                            }
                        }
                    }
                }

                //iframe cleanup appears to randomly cause the download to fail
                //not doing it seems better than failure...
                //if ($iframe) {
                //    $iframe.remove();
                //}

            }, 0);
        }


        function htmlSpecialCharsEntityEncode(str) {
            return str.replace(htmlSpecialCharsRegEx, function(match) {
                return '&' + htmlSpecialCharsPlaceHolders[match];
        	});
        }
        var promise = deferred.promise();
        promise.abort = function() {
            cleanUp();
            $iframe.remove();
        };
        return promise;
    }
});

})(jQuery, this);
