set -x

# Generate a random AES key
openssl rand -hex 64 -out temp_key

# Encrypt the input file with the temp key
# Note that the -salt switch adds the string "Salted__" followed by 8 bytes of random salt at start
# of the output file; use the -nosalt switch to eliminate that behavior.
openssl enc -aes-256-cbc -salt -in input/cleartext -pass file:./temp_key -out temp_encrypted_data
openssl base64 -A -in temp_encrypted_data -out transport/encrypted_data.b64

# Encrypt the temp key with the public key file
openssl rsautl -encrypt -pubin -inkey keys/public.pem -in temp_key -out temp_encrypted_key
openssl base64 -A -in temp_encrypted_key -out transport/encrypted_key.b64

# Delete the tmporary encryption key
# rm temp_encrypted_data
rm temp_encrypted_key
rm temp_key
