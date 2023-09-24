//function loginPopup() {
//    var loginPopup = document.getElementById("loginPopup");
//    if (loginPopup.style.display == "none") {
//        loginPopup.style.display = "block";
//    } else {
//        loginPopup.style.display = "none";
//    }
//}

var navLinks = document.querySelectorAll("nav a");
for (var i = 0; i < navLinks.length; i++) {
	var link = navLinks[i]
	if (link.getAttribute('href') == window.location.pathname) {
		link.classList.add("live");
		break;
	}
}

var elements = document.getElementsByClassName("photo-flex")

for (let i=0; i < elements.length; i++) {
    elements[i].addEventListener('wheel', (event) => {
        event.preventDefault();

        //sideScroll(elements[i], event.deltaY < 0 ?'left':'right', 25, 100, 10);
        elements[i].scrollBy({
            left: event.deltaY < 0 ? -100 : 100,
            //behavior: 'smooth',
        });
    });
}

function sideScroll(element,direction,speed,distance,step){
    scrollAmount = 0;
    var slideTimer = setInterval(function(){
        if(direction == 'left'){
            element.scrollLeft -= step;
        } else {
            element.scrollLeft += step;
        }
        scrollAmount += step;
        if(scrollAmount >= distance){
            window.clearInterval(slideTimer);
        }
    }, speed);
}
