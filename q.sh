#!/bin/bash

# make release
# sudo rm /usr/local/bin/tsh
# sudo rm /usr/local/bin/teleport
# sudo rm /usr/local/bin/tctl

# sudo cp build/teleport /usr/local/bin
# sudo cp build/tsh /usr/local/bin
# sudo cp build/tctl /usr/local/bin


make -C build.assets release
sudo ssh -i ~/.ssh/quin.pem ec2-user@ec2-18-236-68-138.us-west-2.compute.amazonaws.com sudo rm tctl
sudo ssh -i ~/.ssh/quin.pem ec2-user@ec2-18-236-68-138.us-west-2.compute.amazonaws.com sudo rm tsh
sudo ssh -i ~/.ssh/quin.pem ec2-user@ec2-18-236-68-138.us-west-2.compute.amazonaws.com sudo rm teleport
echo "completed removing"
sudo scp -i ~/.ssh/quin.pem build/teleport ec2-user@ec2-18-236-68-138.us-west-2.compute.amazonaws.com:~/
sudo scp -i ~/.ssh/quin.pem build/tctl ec2-user@ec2-18-236-68-138.us-west-2.compute.amazonaws.com:~/
sudo scp -i ~/.ssh/quin.pem build/tsh ec2-user@ec2-18-236-68-138.us-west-2.compute.amazonaws.com:~/
echo "complete adding"
# sudo rm /usr/bin/tsh
# sudo rm /usr/bin/teleport
# sudo rm /usr/bin/tctl

# sudo cp teleport /usr/bin
# sudo cp tsh /usr/bin
# sudo cp tctl /usr/bin

# sudo systemctl restart teleport

# sudo ssh -i ~/.ssh/jane.pem  ec2-user@ec2-3-137-180-183.us-east-2.compute.amazonaws.com ./q.sh

# sdfkhjsdlf

# #!/bin/bash
# # add 
# sudo scp -i ~/.ssh/jane.pem teleport ec2-user@ec2-3-21-98-20.us-east-2.compute.amazonaws.com:~/
# sudo scp -i ~/.ssh/jane.pem tsh ec2-user@ec2-3-21-98-20.us-east-2.compute.amazonaws.com:~/
# sudo scp -i  ~/.ssh/jane.pem tctl ec2-user@ec2-3-21-98-20.us-east-2.compute.amazonaws.com:~/
# sudo ssh -i ~/.ssh/jane.pem  ec2-user@ec2-3-21-98-20.us-east-2.compute.amazonaws.com ./q.sh
# # add
# sudo scp -i  ~/.ssh/jane.pem teleport ec2-user@ec2-18-216-72-177.us-east-2.compute.amazonaws.com:~/
# sudo scp -i  ~/.ssh/jane.pem tsh ec2-user@ec2-18-216-72-177.us-east-2.compute.amazonaws.com:~/
# sudo scp -i  ~/.ssh/jane.pem tctl ec2-user@ec2-18-216-72-177.us-east-2.compute.amazonaws.com:~/
# sudo ssh -i ~/.ssh/jane.pem  ec2-user@ec2-18-216-72-177.us-east-2.compute.amazonaws.com ./q.sh
# # scp to auth servers 
# sudo scp -i ~/.ssh/jane.pem teleport ec2-user@172.31.1.146:~/
# sudo scp -i ~/.ssh/jane.pem tsh ec2-user@172.31.1.146:~/
# sudo scp -i  ~/.ssh/jane.pem tctl ec2-user@172.31.1.146:~/
# sudo ssh -i ~/.ssh/jane.pem  ec2-user@172.31.1.146 ./q.sh
# # add
# sudo scp -i  ~/.ssh/jane.pem teleport ec2-user@172.31.0.236:~/
# sudo scp -i  ~/.ssh/jane.pem tsh ec2-user@172.31.0.236:~/
# sudo scp -i  ~/.ssh/jane.pem tctl ec2-user@172.31.0.236:~/
# sudo ssh -i ~/.ssh/jane.pem  ec2-user@172.31.0.236 ./q.sh




#!/bin/bash

# # scp to auth servers 
# sudo scp -i ~/.ssh/jane.pem teleport ec2-user@172.31.1.146:~/
# sudo scp -i ~/.ssh/jane.pem tsh ec2-user@172.31.1.146:~/
# sudo scp -i  ~/.ssh/jane.pem tctl ec2-user@172.31.1.146:~/
# sudo ssh -i ~/.ssh/jane.pem  ec2-user@172.31.1.146 ./q.sh
# # add
# sudo scp -i  ~/.ssh/jane.pem teleport ec2-user@172.31.0.236:~/
# sudo scp -i  ~/.ssh/jane.pem tsh ec2-user@172.31.0.236:~/
# sudo scp -i  ~/.ssh/jane.pem tctl ec2-user@172.31.0.236:~/
# sudo ssh -i ~/.ssh/jane.pem  ec2-user@172.31.0.236 ./q.sh

# # add 
# sudo scp -i ~/.ssh/jane.pem teleport ec2-user@ec2-3-21-98-20.us-east-2.compute.amazonaws.com:~/
# sudo scp -i ~/.ssh/jane.pem tsh ec2-user@ec2-3-21-98-20.us-east-2.compute.amazonaws.com:~/
# sudo scp -i  ~/.ssh/jane.pem tctl ec2-user@ec2-3-21-98-20.us-east-2.compute.amazonaws.com:~/
# sudo ssh -i ~/.ssh/jane.pem  ec2-user@ec2-3-21-98-20.us-east-2.compute.amazonaws.com ./q.sh
# # add
# sudo scp -i  ~/.ssh/jane.pem teleport ec2-user@ec2-18-216-72-177.us-east-2.compute.amazonaws.com:~/
# sudo scp -i  ~/.ssh/jane.pem tsh ec2-user@ec2-18-216-72-177.us-east-2.compute.amazonaws.com:~/
# sudo scp -i  ~/.ssh/jane.pem tctl ec2-user@ec2-18-216-72-177.us-east-2.compute.amazonaws.com:~/
# sudo ssh -i ~/.ssh/jane.pem  ec2-user@ec2-18-216-72-177.us-east-2.compute.amazonaws.com ./q.sh

