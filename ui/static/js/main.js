function loginPopup() {
    var loginPopup = document.getElementById("loginPopup");
    if (loginPopup.style.display == "none") {
        loginPopup.style.display = "block";
    } else {
        loginPopup.style.display = "none";
    }
}

var navLinks = document.querySelectorAll("nav a");
for (var i = 0; i < navLinks.length; i++) {
	var link = navLinks[i]
	if (link.getAttribute('href') == window.location.pathname) {
		link.classList.add("live");
		break;
	}
}
