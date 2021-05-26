set -x

# Decrypt the temporary key with private key
openssl base64 -A -d -in transport/encrypted_key.b64 -out temp_encrypted_key
openssl rsautl -decrypt -inkey keys/private.pem -in temp_encrypted_key -out temp_key

# Decrypt the input file
# Note that the -salt switch removes 16 bytes of salt from the beginning of the encrypted data;
# the -nosalt switch is usedto eliminate this behavior.
openssl base64 -A -d -in transport/encrypted_data.b64 -out temp_encrypted_data
openssl enc -d -aes-256-cbc -salt -in temp_encrypted_data -pass file:./temp_key -out output/cleartext

# Clean up
rm temp_encrypted_data
rm temp_encrypted_key
rm temp_key

