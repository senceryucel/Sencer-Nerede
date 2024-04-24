<div align="center">

# Sencer Nerede</div>


### Read First
This mini project was my playground for getting my hands warmed up with Golang.

### Başlamadan
Bu mini proje, Golang'e alışmak için aklıma gelen bir fikrin uygulanışıydo.




- [Türkçe](#Türkçe)
    - [Sencer Nerede nedir?](#sencer-nerede-nedir)
    - [Nasıl çalışır?](#nasıl-çalışır)
        - [Konumları Gösterme](#konumları-gösterme)
        - [Konumu Güncelleme](#konumu-güncelleme)
- [English](#english)
  - [What is Sencer Nerede?](#what-is-sencer-nerede)
  - [How does it work?](#how-does-it-work)
    - [Showing Locations](#showing-locations)
    - [Updating Location](#updating-location)

<br><br>


<div align="center">

## Türkçe </div>

### Sencer Nerede nedir?
Sencer Nerede, kendi güncel konumumu herkese açık olarak yayınlamak için geliştirdiğim bir projedir. Konumum, güncellediğim sürece, herkese açık bir haritada gösterilir.

### Nasıl çalışır?

#### Konumları Gösterme 
Yeni bir session oluşunca Go server'ı Redis DB içinde olan konum verilerini alıyor ve bu verileri websocket üzerinden frontend'e gönderiyor. Frontend, bu verileri bir Leaflet haritasında gösteriyor.<br>
#### Konumu Güncelleme
Bir Nginx web sunucusu, reverse proxy olarak website üzerinden alınan HTTPS isteklerini localde çalışan bir HTTP sunucusuna (Go) yönlendiriyor. Server, farklı noktalar için konum verilerini döndürecek ve kabul edecek şekilde çalışıyor (konum güncellemelerini sadece ben, [update endpoint'i](https://senceryucel.com/update.html) üzerinden authentication ile yapabiliyorum). Auth doğru ise browser üzerinden geolocation ile konum alınıyor, konum bir MQTT broker'dan geçerek Redis DB'ye yazılıyor. MQTT kullanma amacı, gelecek süreçte geliştireceğim ufak bir GPS Tracker cihazını da sisteme kolayca entegre etmek.

Frontend, basit bir html. Proje şu anda bana ait bir droplet'te çalışıyor. [Buradan](https://senceryucel.com/nerede) erişebilirsiniz.


<br>

<div align="center"> 

## English </div>



### What is Sencer Nerede?
I am Sencer. Sencer Nerede (Where is Sencer) is a project that I have developed to publicly share my current location. As long as I keep updating my location, it shows it on a map.

### How does it work?

#### Showing Locations
Whenever a new session is created, the Go server gets the location data from Redis DB and sends it to the frontend via websocket. The frontend shows this data on a Leaflet map.<br>
#### Updating Location
Golang is used in the backend and raw javascript is used in the frontend. It employs an Nginx web server to serve as a reverse proxy, redirecting HTTPS requests received on the website to a locally running HTTP server (Go). The backend is an http server that returns and accepts location data for different endpoints (only I can update the location via [update endpoint](https://senceryucel.com/update.html) with credentials). 
If the credentials is correct, the location is obtained through geolocation over the browser, and this location is passed through an MQTT broker before being written to Redis DB. The reason for using MQTT is to easily integrate a small GPS Tracker device that I might develop into the system in the future.

The frontend is a simple html page. The project is currently running on a droplet of mine. You can access it from [here](https://senceryucel.com/nerede).
