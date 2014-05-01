package main

import (
	"math/rand"
)

func init() {
	r := rand.New(rand.NewSource(99))
}

func my_random() {
   seed = seed*1664525UL + 1013904223UL; // numerical recipes, 32 bit

   switch((seed>>30) & 3)
   {
   case 0: return 0;
   case 1: return -1;
   case 2: return 1;
   case 3: return 0;
   }
   return 0;
}

func allzero(x *[]int) bool {
   for i := range x {
   	if i != 0 {
   		return false
   	}
   }
   return true
}

func convolve(out *[]int, v1 *[]int, v2 *[]int){
	outlen := len(out)
	v1len := len(v1)
	v2len := len(v2)
	var result int

	for i := 0; i < outlen; i++ {
		result = 0
		for j := 0; j < v2len; j++ {
			result += v1[i+j]*v2[j]
		}
		out[i] = result
	}
}

void advance(vector<int>& v)
{
   for(auto &x : v)
   {
      if(x==-1)
      {
         x = 1;
         return;
      }
      x = -1;
   }
}

void convolve_random_arrays(void)
{
   const size_t n = 6;
   const int two_to_n_plus_one = 128;
   const int iters = 1000;
   int bothzero = 0;
   int firstzero = 0;

   vector<int> S(n+1);
   vector<int> F(n);
   vector<int> FS(2);

   time_t current_time;
   time(&current_time);
   seed = current_time;

   for(auto &x : S)
   {
      x = -1;
   }
   for(int i=0; i<two_to_n_plus_one; ++i)
   {
      for(int j=0; j<iters; ++j)
      {
         do
         {
            for(auto &x : F)
            {
               x = my_random();
            }
         } while(allzero(F));
         convolve(FS, S, F);
         if(FS[0] == 0)
         {
            firstzero++;
            if(FS[1] == 0)
            {
               bothzero++;
            }
         }
      }
      advance(S);
   }
   cout << firstzero << endl; // This output can slow things down
   cout << bothzero << endl; // comment out for timing the algorithm
}