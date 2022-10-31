// Returns navigator to be used by the game script in the iframe
var connectSerial = async function () {
  return navigator;
};

DOM.Ready(function () {
  // Handle theme
  window.onload = HandleDarkMode();

  // Perform AJAX post on click on method=post|delete anchors
  ActivateMethodLinks();

  // Insert CSRF tokens into forms
  ActivateForms();

  ActivateCopyApprovedEmail();
  ActivateLoginInput();
});

// Toggles the theme
function ToggleDarkMode() {
  let bodyTag = document.getElementsByTagName("body")[0];
  let toggleTag = document.getElementById("colorToggle");

  if (bodyTag.classList.contains("lightMode")) {
    bodyTag.classList.replace("lightMode", "darkMode");
    toggleTag.innerHTML = "Light Mode";
    setCookie("theme", "dark");
  } else {
    bodyTag.classList.replace("darkMode", "lightMode");
    toggleTag.innerHTML = "Dark Mode";
    setCookie("theme", "light");
  }
}

// Gets the theme from the cookie and sets the theme
function HandleDarkMode() {
  let toggleTag = document.getElementById("colorToggle");
  let bodyTag = document.getElementsByTagName("body")[0];
  // Not setting dark mode automatically due to the bug in Chromium on Linux.
  /*   
     if (
      window.matchMedia &&
      window.matchMedia("(prefers-color-scheme: dark)").matches
    ) {
      bodyTag.classList.add("darkMode");
      toggleTag.innerHTML = "Light Mode";
    } else {
      bodyTag.classList.add("lightMode");
      toggleTag.innerHTML = "Dark Mode";
    } */

  theme = getCookie("theme");

  if (theme) {
    if (theme === "light") {
      bodyTag.classList.add("lightMode");
      toggleTag.innerHTML = "Dark Mode";
    } else {
      bodyTag.classList.add("darkMode");
      toggleTag.innerHTML = "Light Mode";
    }
  } else {
    bodyTag.classList.add("lightMode");
    toggleTag.innerHTML = "Dark Mode";
  }

  toggleTag.addEventListener("click", ToggleDarkMode);
}

// Insert an input into every form with js to include the csrf token.
// this saves us having to insert tokens into every form.
function ActivateForms() {
  // Get authenticity token from head of page
  var token = authenticityToken();

  DOM.Each("form", function (f) {
    // Create an input element
    var csrf = document.createElement("input");
    csrf.setAttribute("name", "authenticity_token");
    csrf.setAttribute("value", token);
    csrf.setAttribute("type", "hidden");

    //Append the input
    f.appendChild(csrf);
  });
}

function authenticityToken() {
  // Collect the authenticity token from meta tags in header
  var meta = DOM.First("meta[name='authenticity_token']");
  if (meta === undefined) {
    e.preventDefault();
    return "";
  }
  return meta.getAttribute("content");
}

function ActivateCopyApprovedEmail() {
  DOM.On(".copy_button", "click", function (e) {
    /* Get the text field */
    var copyText = document.getElementById("approvedEmail");

    /* Select the text field */
    copyText.select();
    copyText.setSelectionRange(0, 99999); /* For mobile devices */

    /* Copy the text inside the text field */
    document.execCommand("copy");
  });
}

function ActivateLoginInput() {
  DOM.On(".code", "input", function (e) {
    var target = e.target,
      position = target.selectionEnd,
      length = target.value.length;

    target.value = target.value
      .replace(/[^\dA-Z]/g, "")
      .replace(/(.{4})/g, "$1 ")
      .trim();
    target.selectionEnd = position +=
      target.value.charAt(position - 1) === " " &&
      target.value.charAt(length - 1) === " " &&
      length !== target.value.length
        ? 1
        : 0;
  });
}

// Perform AJAX post on click on method=post|delete anchors
function ActivateMethodLinks() {
  DOM.On('a[method="post"]', "click", function (e) {
    var link = this;

    // Ignore disabled links
    if (DOM.HasClass(link, "disabled")) {
      e.preventDefault();
      return false;
    }

    // Get authenticity token from head of page
    var token = authenticityToken();

    // Perform a post to the specified url (href of link)
    var url = link.getAttribute("href");
    var data = "authenticity_token=" + token;

    DOM.Post(
      url,
      data,
      function (request) {
        // Use the response url to redirect
        window.location = request.responseURL;
      },
      function (request) {
        // Respond to error
        console.log("error", request);
      }
    );

    e.preventDefault();
    return false;
  });

  DOM.On('a[method="back"]', "click", function (e) {
    history.back(); // go back one step in history
    e.preventDefault();
    return false;
  });
}

// Gets the cookie from the browser
function getCookie(cname) {
  let name = cname + "=";
  let decodedCookie = decodeURIComponent(document.cookie);
  let ca = decodedCookie.split(";");
  for (let i = 0; i < ca.length; i++) {
    let c = ca[i];
    while (c.charAt(0) == " ") {
      c = c.substring(1);
    }
    if (c.indexOf(name) == 0) {
      return c.substring(name.length, c.length);
    }
  }
  return "";
}

// Sets the cookie in the browser
function setCookie(cname, cvalue, exdays) {
  let expires;
  if (exdays) {
    const d = new Date();
    d.setTime(d.getTime() + exdays * 24 * 60 * 60 * 1000);
    expires = "expires=" + d.toUTCString();
  } else {
    expires = "Tue, 19 Jan 2038 04:14:07 GMT";
  }

  document.cookie = cname + "=" + cvalue + ";" + expires + ";path=/";
}
