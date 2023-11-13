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

// Select images for deletion
var selected = []

function listenLeftClick() {
    toggleSelected(this);
    return false;
}

function toggleSelected(e) {
    var img_src = e.getAttribute('src').split("/"); 
    var file = img_src[img_src.length-1];

    var delButton = document.getElementById("delButton");
    var downloadButton = document.getElementById("downloadButton");

    var index = selected.indexOf(file);
    //was already selected
    if (index !== -1) {
        //was the only selected
        if (selected.length == 1) {
            delButton?.classList.toggle("hidden");
            downloadButton.classList.toggle("hidden");
            images = document.getElementsByClassName("photo-grid-item");

            //remove left click listener for all images
            for (i=0; i<images.length; i++) {
                images[i].removeEventListener('click', listenLeftClick)
                images[i].onclick=function() {return true;};
                //the image that is being clicked on needs to have its onclick attribute
                //set after the current click
                e.onclick=function() {this.onclick=()=>{return true}; return false;};
            }
        }
        selected.splice(index, 1);
    } else { //wasn't selected
        //is the first to be selected
        if (selected.length == 0) {
            delButton?.classList.toggle("hidden");
            downloadButton.classList.toggle("hidden");
            images = document.getElementsByClassName("photo-grid-item");

            //add left click listener for all images
            for (i=0; i<images.length; i++) {
                images[i].addEventListener('click', listenLeftClick);
                images[i].onclick=function() {return false;};
            }
        }
        selected.push(file);
    }

    e.classList.toggle("selected");
}

function deleteSelected(event, token) {
    if (!confirm('Sei sicuro di voler cancellare le foto?')) {
        return
    }

    var data = {
        event: event,
        photos: selected,
        csrf_token: token
    };
    fetch("/photos/delete", {
        method: "POST",
        headers: {'Content-Type': 'application/json'},
        body: JSON.stringify(data),
        redirect: "follow"
    }).then(res => {
            console.log("Request complete, response:", res);
            location.reload();
    })
}

function downloadSelected(event, token) {
    if (!confirm('Sei sicuro di voler cancellare le foto?')) {
        return
    }

    var data = {
        event: event,
        photos: selected,
        csrf_token: token
    };
    fetch("/photos/download", {
        method: "POST",
        headers: {'Content-Type': 'application/json'},
        body: JSON.stringify(data),
        redirect: "follow"
    })
    .then(response => {
        const header = response.headers.get('Content-Disposition');
        const parts = header.split(';');
        filename = parts[1].split('=')[1].replaceAll("\"", "");

        return response.blob();
    })
    .then(data => {
        var a = document.createElement("a");
        a.href = window.URL.createObjectURL(data);
        a.download = filename;
        a.click();
    });
}

if (document.getElementById("map") == null) {
    var pmig = document.getElementsByClassName('photo-map-info-grid');
    for(i = 0; i < pmig.length; i++) {
        pmig[i].style.gridTemplateColumns = '100%';
    }
}
// Map
var lat = document.getElementById("latitude")
var lon = document.getElementById("longitude")

if (lat != null && lon != null) {
    console.log(lat)
    console.log(lat.textContent)
    lat = parseFloat(lat.textContent)
    lon = parseFloat(lon.textContent)
    console.log(lat)

    var map = L.map('map', { dragging: !L.Browser.mobile });

    L.tileLayer('https://tile.openstreetmap.org/{z}/{x}/{y}.png', {
        maxZoom: 19,
        attribution: '&copy; <a href="http://www.openstreetmap.org/copyright">OpenStreetMap</a>'
    }).addTo(map);

    var marker = L.marker([lat, lon]).addTo(map);
    map.setView([lat, lon], 15);
    marker.bindPopup("<b>Hello world!</b><br>I am a popup.");
}

// Zoom
image = document.getElementById("FullPhoto")
if (image != null) {
    const viewer = new ImageViewer.FullScreenViewer();
    image.addEventListener("click", function(ev) {
    const imgSrc = image.src;
    const highResolutionImage = image.getAttribute("data-high-res-src");
    viewer.show(imgSrc, highResolutionImage);
  });
}
