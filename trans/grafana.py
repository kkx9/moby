import docker

client = docker.from_env()
images = client.images.list(name="grafana")

# Create a dictionary to store unique image IDs, their history, and layer sizes
unique_images = {}

# Loop through each image and get its history and layer sizes
for i in range(len(images)-1, -1, -1):
    image = images[i]
    image_id = image.id.split(":")[1]
    if image_id not in unique_images:
        unique_images[image_id] = {"history": [], "layer_sizes": []}
    history = image.history()
    for j in range(len(history)-2, -1, -1):
        layer = history[j]
        layer_id = layer['Id'].split(":")[1]
        if layer_id not in unique_images[image_id]["history"]:
            unique_images[image_id]["history"].append(layer_id)
            unique_images[image_id]["layer_sizes"].append(layer["Size"])

# Write the image IDs, their history, and layer sizes to a file
with open("grafana_history.txt", "w") as f:
    count = 0
    for image_id, data in unique_images.items():
        count += 1
        f.write(f"Image ID: {image_id}\n")
#        f.write("History:\n")
        for i, layer in enumerate(data["history"]):
            f.write(f"{layer},{data['layer_sizes'][i]}\n")
#            f.write(f"  Layer ID: {layer}, Size: {data['layer_sizes'][i]}\n")
#        f.write("\n")
print(count)
