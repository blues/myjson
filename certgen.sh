openssl req -newkey rsa:4096 -nodes -sha512 -x509 -days 3650 -nodes -out ~/cert/public.pem -keyout ~/cert/private.pem
cp ~/cert/public.pem ~/cert/certifier.pem
