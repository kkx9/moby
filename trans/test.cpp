
#include <fstream>
#include <iostream>
using namespace std;
 
int main ()
{
    
   char data[100];
   ofstream outfile;
   outfile.open("afile.dat");

   outfile << "hello twice1" << endl;
 
   outfile.close();
   return 0;
}
